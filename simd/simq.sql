-- MySQL dump 10.13  Distrib 8.0.30, for macos12 (arm64)
--
-- Host: localhost    Database: simq
-- ------------------------------------------------------
-- Server version	8.0.30

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `Queue`
--

DROP TABLE IF EXISTS `Queue`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `Queue` (
  `SID` bigint NOT NULL AUTO_INCREMENT,
  `File` varchar(80) NOT NULL,
  `Username` varchar(40) NOT NULL,
  `Name` varchar(80) NOT NULL DEFAULT '',
  `Priority` int NOT NULL DEFAULT '5',
  `Description` varchar(256) NOT NULL DEFAULT '',
  `MachineID` varchar(80) NOT NULL DEFAULT '',
  `URL` varchar(80) NOT NULL DEFAULT '',
  `State` int NOT NULL DEFAULT '0',
  `DtEstimate` datetime DEFAULT NULL,
  `DtCompleted` datetime DEFAULT NULL,
  `Created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `Modified` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`SID`)
) ENGINE=InnoDB AUTO_INCREMENT=6 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `Queue`
--

LOCK TABLES `Queue` WRITE;
/*!40000 ALTER TABLE `Queue` DISABLE KEYS */;
INSERT INTO `Queue` VALUES (2,'sm.json5','stevemansour','Development Default Simulation',5,'','CCE57473-4791-5E21-977E-F7E2B9145337','',1,NULL,NULL,'2024-07-08 06:28:44','2024-07-08 17:20:18'),(3,'config.json5','stevemansour','Development Default Simulation',5,'','CCE57473-4791-5E21-977E-F7E2B9145337','',1,NULL,NULL,'2024-07-08 06:28:51','2024-07-08 17:20:38'),(4,'config.json5','stevemansour','Dev sim',5,'','ABCDEFA--2334-3454-8484-ABCD23459939','',3,NULL,NULL,'2024-07-09 06:28:51','2024-07-09 15:37:04'),(5,'sm.json5','stevemansour','Dev sim long',5,'','CCE57473-4791-5E21-977E-F7E2B9145337','',2,NULL,NULL,'2024-07-08 06:28:51','2024-07-09 15:38:54');
/*!40000 ALTER TABLE `Queue` ENABLE KEYS */;
UNLOCK TABLES;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2024-07-09  8:39:26
