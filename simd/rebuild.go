package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stmansour/simq/data"
	"github.com/stmansour/simq/util"
)

// RebuildSimulatorList the simulator list for this machine.  This is called
// when simd starts and makes the assumption that simd may have been killed
// or crashed or shut down irregularly in some way. This will rebuild
// the list of jobs it needs to see through.
// -----------------------------------------------------------------------------
func RebuildSimulatorList() error {
	var err error
	var dataBytes []byte
	//----------------------------------------------------------------------
	// DOES DISPATCHER HAVE ANY "IN-PROGRESS" SIMULATIONS FOR THIS MACHINE?
	//----------------------------------------------------------------------
	cmd := util.Command{
		Command:  "GetMachineQueue",
		Username: "simd",
	}
	var cmdDataStruct struct {
		MachineID string
	}
	if cmdDataStruct.MachineID, err = util.GetMachineUUID(); err != nil {
		return fmt.Errorf("failed to get machine ID: %v", err)
	}
	if dataBytes, err = json.Marshal(cmdDataStruct); err != nil {
		return fmt.Errorf("failed to marshal book request: %v", err)
	}
	cmd.Data = json.RawMessage(dataBytes)
	body := util.SendRequest(app.cfg.FQDispatcherURL, &cmd)

	//-----------------------------------------------------------
	// UNMARSHAL THE RESPONSE.  This gives us the list of jobs
	// that the dispatcher thinks we're working on...
	//-----------------------------------------------------------
	resp := struct {
		Status string
		Data   []data.QueueItem
	}{}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		if strings.Contains(err.Error(), "unexpected end of JSON input") {
			log.Printf("dispatcher has no jobs currently pending on this machine\n")
			return nil
		}
		log.Printf("Error unmarshaling response: %v\n", err)
	}

	//-----------------------------------------------------------------
	// ARE THERE ANY SIMULATIONS IN THE SIMULATION DIRECTORY?
	// simd keeps its simulations in ./simulations/<SID>
	//-----------------------------------------------------------------
	type DIR struct {
		Dir          string
		InDispatcher bool
	}
	var dirs []DIR
	dirPath := filepath.Join(app.cfg.SimdSimulationsDir, "simulations")
	dir, err := os.ReadDir(dirPath)
	if err != nil {
		log.Fatal(err)
	}
	for _, entry := range dir {
		if entry.IsDir() {
			var d = DIR{
				Dir:          entry.Name(),
				InDispatcher: false,
			}
			dirs = append(dirs, d)
		}
	}

	//------------------
	// DEBUG
	//------------------
	if len(dirs) > 0 {
		log.Printf("SIMD: dispatcher reports %d simulations belonging to this machine\n", len(resp.Data))
		s := "      SIDs = "
		for i := 0; i < len(resp.Data); i++ {
			s += fmt.Sprintf("%d ", resp.Data[i].SID)
		}
		log.Printf("%s\n", s)
		log.Printf("SIMD: these SID simulation directories are in simd's simulation directory:\n")
		s = "      DIRS = "
		for i := 0; i < len(dirs); i++ {
			s += fmt.Sprintf("%s ", dirs[i].Dir)
		}
		log.Printf("%s\n", s)
	}

	//---------------------------------------------------------------------------
	// WHAT JOBS IN THE SIMULATION DIRECTORY WERE ALSO LISTED BY THE DISPATCHER?
	//---------------------------------------------------------------------------
	for i := 0; i < len(resp.Data); i++ {
		for j := 0; j < len(dirs); j++ {
			sid, err := strconv.ParseInt(dirs[j].Dir, 10, 64)
			if err != nil {
				continue // not a number
			}
			if resp.Data[i].SID == sid {
				log.Printf("Found simulation to recover: %d\n", resp.Data[i].SID)
				dirs[j].InDispatcher = true // this dispatcher simulation is in our simulations directory
				break
			}
		}
	}

	//---------------------------------------------------------------------
	// DELETE THE SIMULATIONS IN THE SIMULATION DIRECTORY THAT ARE NOT IN
	// THE DISPATCHER LIST.  The dispatcher may have given up on us and
	// rebooked the simulation with another machine. There may be other
	// ways for this to happen, but in any case, if there are simulations
	// in our local directory that are not in the dispatcher list, we can
	// delete them.
	//---------------------------------------------------------------------
	for i := 0; i < len(dirs); i++ {
		if !dirs[i].InDispatcher {
			log.Printf("Deleting simulation not found in dispatcher: %s\n", dirs[i].Dir)
			dir := filepath.Join(app.cfg.SimdSimulationsDir, "simulations", dirs[i].Dir)
			os.RemoveAll(dir)
		}
	}

	//-------------------------------------------------------------------------
	// ANALYZE AND TRY TO RECOVER THE REMAINING ITEMS IN THE DISPATCHER'S LIST
	//-------------------------------------------------------------------------
	for i := 0; i < len(resp.Data); i++ {
		switch resp.Data[i].State {
		case data.StateBooked:
			recoverBookedSimulation(&resp.Data[i])
		case data.StateExecuting:
			recoverExecutingSimulation(&resp.Data[i])
		case data.StateCompleted:
			recoverArchiveSimResults(&resp.Data[i])
		}
	}

	return nil
}

// recoverExecutingSimulation - In this case, the simulation was booked and
// the simulator was executing enough to get that first message to dispatcher
// which put it into the Executing state.  We need to either re-attach to the
// existing simulator or restart it.
// ------------------------------------------------------------------------------
func recoverExecutingSimulation(qi *data.QueueItem) {
	log.Printf("simd >>>> RECOVER EXECUTING SIMULATION - attempting to recover an executing simulation, sid = %d\n", qi.SID)
	//----------------------------------------------
	// IS THE SIMULATOR FOR THIS JOB STILL RUNNING?
	//----------------------------------------------
	sim := buildSimFromQueueItem(qi)
	if sim.FindRunningSimulator() {
		log.Printf("simd >>>> connected with running simulator for sid = %d\n", sim.SID)
		go monitorSimulator(&sim)
		return
	}

	//---------------------------------------------------------------------------
	// UNFORTUNATELY, THE SIMULATOR IS NOT RUNNING. NEXT THING IS TO CHECK AND
	// SEE IF IT FINISHED. HAVE A LOOK AT THE FILES IN ITS DIRECTORY
	//---------------------------------------------------------------------------
	if recovered, err := sim.recoverBasedOnFiles(); err != nil || recovered {
		return // error occurred and logged
	}

	//----------------------------------------------------------------
	// IF THE FILES ARE NOT THERE, WE NEED TO RESTART THE SIMULATOR
	//----------------------------------------------------------------
	log.Printf("simd >>>> rebooking sid = %d\n", sim.SID)
	bookAndRunSimulation("Rebook", sim.SID)
}

// recoverArchiveSimResults - In this case, the simulation was apparently
// finished but the results were not archived. Attempt to archive them
// -----------------------------------------------------------------------
func recoverArchiveSimResults(qi *data.QueueItem) {
	log.Printf("simd >>>> RECOVER ARCHIVED SIMULATION RESULTS - attempting to archive simulation results for sid = %d\n", qi.SID)
	sim := buildSimFromQueueItem(qi)

	//--------------------------------
	// SEE IF THE ARCHIVE FILE EXISTS
	//--------------------------------
	files, err := getFilenamesInDir(sim.Directory)
	if err != nil {
		log.Printf("simd >>>> Simulation: %d - error while loading filenames in %s: error: %v\n", sim.SID, sim.Directory, err)
		return
	}
	found := false
	for i := 0; i < len(files); i++ {
		if files[i] == "results.tar.gz" {
			found = true
			break
		}
	}

	//--------------------------------
	// IF IT WASN'T FOUND, CREATE IT
	//--------------------------------
	if !found {
		//-----------------------------------------
		// DO WE HAVE FINREP.CSV or SIMSTATS.CSV
		//-----------------------------------------
		for i := 0; i < len(files); i++ {
			if files[i] == "finrep.csv" || files[i] == "simstats.csv" {
				found = true
				break
			}
		}
		if !found {
			log.Printf("simd >>>> NO RESULTS FROM SIMULATOR FOUND for SID: %d\n", sim.SID)
			log.Printf("simd >>>> REBOOKING SID: %d TO PROPERLY GENERATE RESULTS\n", sim.SID)
			bookAndRunSimulation("Rebook", sim.SID)
			return
		}

		err = sim.archiveSimulationResults()
		if err != nil {
			log.Printf("simd >>>> Simulation: %d - error creating results archive in %s: error: %v\n", sim.SID, sim.Directory, err)
			return
		}
	}

	//--------------------------------
	//  SEND END SIMULATION REQUEST
	//--------------------------------
	if err = sim.sendEndSimulationRequest(); err != nil {
		log.Printf("Failed to send end simulation request: %v", err)
		return
	}

	log.Printf("simd >>>> Simulation: %d - results archived in %s\n", sim.SID, sim.Directory)
}

// recoverBookedSimulation - In this case, the simulation was booked but
// we never go the simulator started.  Try to recover.
// -------------------------------------------------------------------------
func recoverBookedSimulation(qi *data.QueueItem) {
	log.Printf("simd >>>> RECOVER BOOKED SIMULATION - attempting to recover a booked simulation, sid = %d\n", qi.SID)
	sim := buildSimFromQueueItem(qi)

	//-----------------------------------------------------------------
	// Because I've seen it happen, just check to see if the simulator
	// actually finished.  If so, somehow the updates were never sent
	// to the dispatcher.  This may be because the machine lost its
	// network connection. In any case, if we have all the files,
	// just archive them and move on.
	//---------------------------------------------------------------
	if recovered, err := sim.recoverBasedOnFiles(); err != nil || recovered {
		if err != nil {
			if !strings.Contains(err.Error(), "no such file or directory") {
				return // error occurred and logged
			}
			log.Printf("recoverBookedSimulation: %d - error occurred looking for files in %s: error: %v\n", qi.SID, sim.Directory, err)
		}

		if recovered {
			log.Printf("simd >>>> booked simulation recovered\n")
			return
		}
	}

	//------------------------------------------------------------------
	// OK, apparently, the simulator did not complete that simulation.
	// See if we have a config file...
	//------------------------------------------------------------------
	configs, err := findJSON5Files(sim.Directory)
	if err != nil {
		log.Printf("recoverBookedSimulation: %d - error occurred looking for config file in %s: error: %v\n", qi.SID, sim.Directory, err)
	}

	//---------------------------------------------------
	// IF WE DID NOT FIND ANY CONFIG FILES, REBOOK
	//---------------------------------------------------
	if len(configs) == 0 {
		log.Printf("Simulation for SID: %d - Missing config file in directory: %s\n", qi.SID, sim.Directory)
		log.Printf("Rebooking...\n")
		bookAndRunSimulation("Rebook", qi.SID)
		return
	}

	//---------------------------------------------------------------
	// Next, we check for the existence of a "finrep.csv" file. If
	// it exists, then the simulation completed, but the results were
	// not archived. Try to archive them now...
	//---------------------------------------------------------------
	if recovered, err := sim.recoverBasedOnFiles(); err != nil || recovered {
		return // error occurred and logged
	}

	//-----------------------------------------------------------------------
	// No finrep.csv file found. So, either the simulation is still running
	// or the simulator was terminated before the simulation completed.
	// First check to see if the simulation is still running. It does
	// not matter if there is a sim.log file in the directory. The process
	// may still be running or it may have died. We don't know. But, either
	// either way we will need to try to contact the running simulator if it
	// is still running. If it's not running, we'll need to restart it. If
	// it is running, we just need to monitor it.
	//-----------------------------------------------------------------------
	if sim.FindRunningSimulator() {
		go monitorSimulator(&sim)
		return // found the simulator!!
	}

	//----------------------------------------------------------------
	// If we get here, then there is no running simulator assigned to
	// this simulation. So, we need to restart the simulator
	//----------------------------------------------------------------
	bookAndRunSimulation("Rebook", sim.SID)
}

// recoverBasedOnFiles - If the file in the simulation directory indicate that
// the simulation is done, then archive the results.
// ------------------------------------------------------------------------------
func (sim *Simulation) recoverBasedOnFiles() (bool, error) {
	filenames, err := getFilenamesInDir(sim.Directory)
	if err != nil {
		log.Printf("Simulation: %d - error occurred lookng for config file in %s: error: %v\n", sim.SID, sim.Directory, err)
		return false, err
	}
	for i := 0; i < len(filenames); i++ {
		if strings.Contains(filenames[i], "finrep.csv") {
			if err = sim.archiveSimulationResults(); err != nil {
				log.Printf("Simulation: %d - error occurred archiving results: error: %v\n", sim.SID, err)
				return false, err
			}
			if err = sim.sendEndSimulationRequest(); err != nil {
				log.Printf("Simulation: %d - error occurred sending end request: error: %v\n", sim.SID, err)
				return false, err
			}
			log.Printf("Successfully archived results for simulation: %d\n", sim.SID)
			return true, nil
		}
	}

	return false, nil
}

func logNotListening(notlistening []int) {
	if len(notlistening) > 0 {
		s := "nothing listening on ports: "
		for i := 0; i < len(notlistening); i++ {
			s += fmt.Sprintf("%d ", notlistening[i])
		}
		log.Printf("%s\n", s)
	}
}

// FindRunningSimulator - Search for a running simulator that belongs to this simulation
// If it finds the simulator running it will return true. Otherwise it returns false
// If it returns true, then sim.URL will be set
// --------------------------------------------------------------------------------
func (sim *Simulation) FindRunningSimulator() bool {
	//---------------------------------------------
	// SEARCH ALL POSSIBLE PORTS ON THIS MACHINE
	// (use of http://127.0.0.1/ is ok in this case)
	//---------------------------------------------
	notlistening := []int{}
	for port := 8090; port <= 8100; port++ {
		url := fmt.Sprintf("http://127.0.0.1:%d/status", port)
		sid, foundSID, err := FetchSID(url)
		if err != nil {
			if strings.Contains(err.Error(), "connect: connection refused") {
				notlistening = append(notlistening, port)
				continue
			}
			log.Printf("Error searching for sid on port %d: %v", port, err)
			continue
		}
		if foundSID && sim.SID == sid {
			logNotListening(notlistening)
			//----------------------------------------------------
			// FOUND IT!!!
			// The simulator is still running.  Save the URL
			// and continue to monitor it as usual
			//----------------------------------------------------
			log.Printf("simd:  >>>>    **** CONNECTED ****   Connected with running simulatorfor SID = %d on port %d\n", sim.SID, port)
			sim.SimPort = port
			sim.BaseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
			sim.FQSimStatusURL = url
			return true
		}
	}
	logNotListening(notlistening)
	return false
}

// FetchSID sends a request to the provided URL and extracts the "SID" field if it exists.
func FetchSID(url string) (int64, bool, error) {
	// Send the HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return 0, false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check for a successful response code
	if resp.StatusCode != http.StatusOK {
		return 0, false, fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, false, fmt.Errorf("failed to read response body: %w", err)
	}

	// Unmarshal the JSON into a map
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return 0, false, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Extract the "SID" field
	sid, ok := result["SID"].(float64) // JSON numbers are unmarshalled as float64
	if !ok {
		return 0, false, nil // "SID" not present or not a number
	}

	return int64(sid), true, nil
}

func findJSON5Files(directory string) ([]string, error) {
	var files []string
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".json5" {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func getFilenamesInDir(dirPath string) ([]string, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	fileInfos, err := dir.Readdir(-1)
	if err != nil {
		return nil, err
	}

	var filenames []string
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			filenames = append(filenames, fileInfo.Name())
		}
	}

	return filenames, nil
}

// buildSimFromQueueItem - create as much of the Simulation struct as possible
// based on the information in the queue item.
// ------------------------------------------------------------------------------
func buildSimFromQueueItem(qi *data.QueueItem) Simulation {
	dir := filepath.Join(app.cfg.SimdSimulationsDir, "simulations", fmt.Sprintf("%d", qi.SID))
	sim := Simulation{
		SID:        qi.SID,
		Directory:  dir,
		ConfigFile: qi.File,
	}

	return sim
}
