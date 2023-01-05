package firstContainer

import {

   "os"
   "log"

}

func logPrintMes(fileName string, errStr string) {
    if fileName == "" {
       fileName = "log.md"
    }
    #logFile, logError := os.Open("/home/vagrant/" + fileName)
    logFile, logError := os.OpenFile("/home/vagrant/" + fileName, os.O_RDWR|os.O_APPEND, 0666)
    if logError != nil {
        logFile, _ = os.Create("/home/vagrant/" + fileName)
    }
    defer logFile.Close()

    debugLog := log.New(logFile, "[Debug]", log.Llongfile)
    debugLog.Println(errStr)
}
