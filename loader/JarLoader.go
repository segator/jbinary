package loader

import (
	"archive/zip"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func Contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}
type hideWindowsFn func(bool)
func ExecuteJavaApplication(defaultExecutionBehaviour string,forceConsoleBehaviourArgs []string,jvmArguments []string,arguments []string,debugPort int,data *string,fn hideWindowsFn) {
	javaExecutableName:="java"
	if runtime.GOOS == "windows" {
		if defaultExecutionBehaviour == "gui" {
			forcedToConsole:=false
			for _, consoleForceArg := range forceConsoleBehaviourArgs {
				if Contains(arguments,consoleForceArg) {
					forcedToConsole=true
				}
			}
			if !forcedToConsole {
				fn(false)
				defer fn(true)
				javaExecutableName="javaw"
			}

		}
		javaExecutableName=javaExecutableName + ".exe"
	}


	//Create empty folder
	tempWorkFolder, _ := ioutil.TempDir("", "jbinary")
	reader := strings.NewReader(*data)
	zipReader, _ := zip.NewReader(reader, int64(reader.Len()))

	for _, zipFile := range zipReader.File {
		unzipped, _ := unzip(zipFile)
		var extractedFullPath = path.Join(tempWorkFolder, zipFile.Name)
		var extensionFile = filepath.Ext(extractedFullPath)
		if strings.Compare(extensionFile, "zip") != 0 {
			var parentFolder = filepath.Dir(extractedFullPath)
			os.MkdirAll(parentFolder, 0755)
			ioutil.WriteFile(extractedFullPath, unzipped, 0755)
		}
		unzipped=nil
	}
	reader=nil
	zipReader = nil
	*data=""
	data=nil
	runtime.GC()
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	if Contains(arguments,"-debug") {
		jvmArguments = append(jvmArguments,fmt.Sprintf("-agentlib:jdwp=transport=dt_socket,server=y,suspend=y,address=%d",debugPort))
	}
	var javaBin = path.Join(tempWorkFolder, fmt.Sprintf("java/bin/%s",javaExecutableName))
	var jarFile = path.Join(tempWorkFolder, "application.jar")
	applicationArguments:=append([]string{"-jar", jarFile},arguments...)
	commandParameters:=append(jvmArguments,applicationArguments...)
	cmd := exec.Command(javaBin,commandParameters...)
	cmd.Dir=dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	var exitCode = 0
	if err := cmd.Wait(); err != nil {
		exitError := err.(*exec.ExitError)
		ws := exitError.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
	}
	os.RemoveAll(tempWorkFolder)
	os.Exit(exitCode)
}

func unzip(zf *zip.File) ([]byte, error) {
	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return ioutil.ReadAll(rc)
}
