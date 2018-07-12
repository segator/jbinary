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
)

func Black(data string) {
	ExecuteJavaApplication(data)
}

//asdad
func ExecuteJavaApplication(data string) {
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
	var javaBin = path.Join(tempWorkFolder, "java/bin/java.exe")
	var jarFile = path.Join(tempWorkFolder, "application.jar")
	cmd := exec.Command(javaBin, "-jar", jarFile)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()

	if err := cmd.Wait(); err != nil {
		exitError := err.(*exec.ExitError)
		ws := exitError.Sys().(syscall.WaitStatus)
		os.Exit(ws.ExitStatus())
	}

	os.Exit(0)
}

func unzip(zf *zip.File) ([]byte, error) {
	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return ioutil.ReadAll(rc)
}
