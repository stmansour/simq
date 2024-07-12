package util

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"

	json5 "github.com/yosuke-furukawa/json5/encoding/json5"
)

// ExternalResources is used to store sensitive or secret config values
// for gaining access to external resources.
type ExternalResources struct {
	Env                int    // 0 = dev, 1 = qa, 2 = production
	DbUser             string // database user
	DbName             string // database name
	DbPass             string // database password
	DbHost             string // database host
	DbPort             int    // database port
	DbType             string // mysql or postgres or sqlite or ...
	SimResultsDir      string // directory to store simulation results
	DispatcherQueueDir string // where dispatcher stores queued configs
	SimdSimulationsDir string // where simulator stores simulations
}

// Define constant variables for DEV, QA, and PROD as per corrected mapping
const (
	DEV  = 0
	QA   = 1
	PROD = 2
)

// EnvironmentToCode maps an environment input string to a corresponding code.
// func EnvironmentToCode(env string) int {
// 	switch env {
// 	case "DEV":
// 		return DEV
// 	case "QA":
// 		return QA
// 	case "PROD":
// 		return PROD
// 	default:
// 		return -1 // Return -1 or another value to indicate an unknown environment
// 	}
// }

// GetSQLOpenString builds the string to use for opening an sql database.
// Input string is the name of the database:  "accord" for phonebook, "rentroll" for RentRoll
// Returns:  a string to pass to sql.Open()
// =======================================================================================
func (a *ExternalResources) GetSQLOpenString(dbname string) string {
	s := ""
	switch a.Env {
	case DEV: //development
		s = fmt.Sprintf("%s:%s@/%s?charset=utf8&parseTime=True",
			a.DbUser, a.DbPass, dbname)
	case QA:
		s = fmt.Sprintf("%s:%s@/%s?charset=utf8&parseTime=True",
			a.DbUser, a.DbPass, dbname)
	case PROD: //production
		s = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True",
			a.DbUser, a.DbPass, a.DbHost, a.DbPort, dbname)
	default:
		fmt.Printf("Unhandled configuration environment: %d\n", a.Env)
		return ""
	}
	return s
}

// ReadExternalResources reads the contents of extres.json5 and fills the ExternalResources struct.
func ReadExternalResources() (*ExternalResources, error) {
	fname := "extres.json5"
	found := false
	//---------------------------------------------
	// Initialize to something reasonable...
	//---------------------------------------------
	var resources = ExternalResources{
		DbHost: "localhost",
		DbName: "simqtest",
		DbPort: 3306,
		DbType: "mysql",
	}

	//--------------------------------------------------------
	// If there is no extres file, set the DbUser to
	// the currently logged in user and return. Everything
	// should continue to work provided they have acces to
	// the database named 'plato'
	//--------------------------------------------------------

	// check for extres.json5 in the current directory
	if _, err := os.Stat(fname); err != nil {
		if os.IsNotExist(err) {
			// check for it in the executable directory...
			exdir, err := GetExecutableDir()
			if err != nil {
				return &resources, err
			}
			found = true

			fname = filepath.Join(exdir, fname)
			if _, err = os.Stat(fname); err != nil {
				if !os.IsNotExist(err) {
					return &resources, err // error is something other than "doesn't exist"
				}
				found = false
			}
		}

		if !found {
			currentUser, err := user.Current()
			if err != nil {
				return &resources, err
			}
			resources.DbUser = currentUser.Username
			return &resources, nil
		}
	}

	//---------------------------------------
	// The extres file was found.  Use it.
	//---------------------------------------
	file, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Decode the JSON5 data into the ExternalResources struct
	err = json5.NewDecoder(file).Decode(&resources)
	if err != nil {
		return nil, err
	}

	return &resources, nil
}

// LoadConfig reads the contents of config.json5 and fills the ExternalResources struct.
func LoadConfig(extres *ExternalResources, fname string) (*ExternalResources, error) {
	var found bool
	if _, err := os.Stat(fname); err != nil {
		if os.IsNotExist(err) {
			// check for it in the executable directory...
			exdir, err := GetExecutableDir()
			if err != nil {
				return extres, err
			}
			found = true

			fname = filepath.Join(exdir, fname)
			if _, err = os.Stat(fname); err != nil {
				if !os.IsNotExist(err) {
					return extres, err // error is something other than "doesn't exist"
				}
				found = false
			}
		}

		if !found {
			return extres, err
		}
	}

	configFile, err := os.Open(fname)
	if err != nil {
		return extres, fmt.Errorf("failed to open config file: %v", err)
	}
	defer configFile.Close()
	byteValue, err := io.ReadAll(configFile)
	if err != nil {
		return extres, fmt.Errorf("failed to read config file: %v", err)
	}

	//-------------------------------------
	// read into our config struct
	//-------------------------------------
	err = json5.Unmarshal(byteValue, extres)
	if err != nil {
		return extres, fmt.Errorf("failed to unmarshal config data into cfg: %v", err)
	}

	return extres, nil
}

// GetExecutableDir returns the directory containing the executable that started the current process.
func GetExecutableDir() (string, error) {
	// Get the full path of the executable.
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	// Get the directory from the executable path.
	execDir := filepath.Dir(execPath)

	return execDir, nil
}
