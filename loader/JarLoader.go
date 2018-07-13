package loader

import (
	"archive/zip"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"runtime"
)



func ExecuteJavaApplication(arguments []string,data string) {
	//Create empty folder
	tempWorkFolder, _ := ioutil.TempDir("", "jbinary")
	zipReader, _ := zip.NewReader(strings.NewReader(data), int64(len(data)))

	for _, zipFile := range zipReader.File {
		unzipped, _ := unzip(zipFile)

		var extractedFullPath = path.Join(tempWorkFolder, zipFile.Name)
		var extensionFile = filepath.Ext(extractedFullPath)
		if strings.Compare(extensionFile, "zip") == 0 {

		} else {
			var parentFolder = filepath.Dir(extractedFullPath)
			os.MkdirAll(parentFolder, 0755)
			ioutil.WriteFile(extractedFullPath, unzipped, 0755)
		}
	}
	var extension=""
	if runtime.GOOS == "windows" {
		extension=".exe"
	}
	var javaBin = path.Join(tempWorkFolder, "java/bin/java"+extension)
	var jarFile = path.Join(tempWorkFolder, "application.jar")
	commandParameters:=append([]string{"-jar", jarFile},arguments...)
	cmd := exec.Command(javaBin,commandParameters...)
	cmd.Dir,_=os.Getwd()
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
