package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"
	"time"
)

const (
	nameSourceFile = "data.go"
	nameVersionInfoFile = "versioninfo.json"
	nameManifestFile = "manifest.xml"
)

var namePackage string

var (
	flagJavaAppPath        = flag.String("jar", path.Join(".", "application.jar"), "Path of the java application")
	flagDest       = flag.String("build", ".", "The destination path of the generated package.")
	flagPlatform   = flag.String("platform", "windows", "Operating system(linux|windows)")
	flagArchitecture   = flag.String("architecture", "amd64", "Building Architecture (amd64|386)")
	flagJreVersio = flag.String("jre-version", "1.8.0_131", "The destination path of the generated package.")
	flagJavaType = flag.String("java-type", "jre", "Java type jre|jdk")
	flagNoMtime    = flag.Bool("ignore-modtime", false, "Ignore modification times on files.")
	flagNoCompress = flag.Bool("no-compress", false, "Do not use compression to shrink the files.")
	flagJVMArguments = flag.String("jvm-arguments", "", "JVM Arguments")
	flagAppArguments = flag.String("app-arguments", "", "App Arguments")
	flagPkg        = flag.String("output-name", "application", "Name of the generated package")
	flagDebugPort = flag.Int64("jre-debug-port", 21500, "Debug port to listen if the generated binary is executed with cli -debug")
	flagServerURL  = flag.String("java-server-url","https://artifacts.alfresco.com/nexus/content/repositories/public","Server base URL to look for java download")
	flagDefaultDownloadURL="{serverURL}/com/oracle/java/{javaType}/{javaVersion}/{javaType}-{javaVersion}-{platform}{architecture}.tgz"
	jreDownloadURL = flag.String("java-download-link",flagDefaultDownloadURL,"Link where to download java distribution format:"+flagDefaultDownloadURL)
	//https://artifacts.alfresco.com/nexus/content/repositories/public/com/oracle/java/jre/1.8.0_131/jre-1.8.0_131-win64.tgz

	flagWinDescription  = flag.String("win-description", "no description", "Windows Application description")
	flagWinCopyright  = flag.String("win-copyright", "no copyright", "Windows Application copyright")
	flagWinCompany  = flag.String("win-company", "no company", "Windows Application company name")
	flagWinIconPath  = flag.String("win-icon-path", "", "icon path")
	flagWinProductName  = flag.String("win-product-name", "product name", "Windows Application product name")
	flagWinProductVersion  = flag.String("win-product-version", "1.0.0.0", "Windows Application product version")
	flagWinMajorVersion  = flag.String("win-version-major", "1", "Windows Application Major version")
	flagWinMinorVersion = flag.String("win-version-minor", "0", "Windows Application Minor version")
	flagWinPatchVersion = flag.String("win-version-patch", "0", "Windows Application Patch version")
	flagWinBuildVersion = flag.String("win-version-build", "0", "Windows Application Build version")
	flagWinExecutionLevel = flag.String("win-invoker", "asInvoker", "Windows Invoker type  asInvoker|requireAdministrator default(asInvoker)")
	flagWinExecutionBehaviour = flag.String("win-execution-behaviour", "console", "Default behaviour to run app, in gui mode no console is shown but you can't execute by console or capture stdout, (console|gui) default(console)")
	flagWinExecutionBehaviourConsoleArgs = flag.String("win-execution-enable-console-behaviour-args", "-console;-terminal", "Arguments that will force console mode in case of default behaviour gui, default (-console;-terminal)")


)

// mtimeDate holds the arbitrary mtime that we assign to files when
// flagNoMtime is set.
var mtimeDate = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)

func main() {
	flag.Parse()
	javaAppPathAbs,_:=filepath.Abs(*flagJavaAppPath)
	namePackage = *flagPkg
	//Create work dir
	tempWorkFolder, _ :=ioutil.TempDir("", "jbinary")
	fmt.Printf("Created Working folder %s\n",tempWorkFolder)
	os.MkdirAll(tempWorkFolder, 0755)
	downloadJRE(tempWorkFolder)
	fmt.Printf("Java application:%s\n",javaAppPathAbs)
	Copy(javaAppPathAbs,filepath.Join(tempWorkFolder,"application.jar"))

	fmt.Println("Generating golang source class")
	file, err := generateSource(*flagJVMArguments,*flagAppArguments,tempWorkFolder)
	if err != nil {
		exitWithError(err,1)
	}

	destDir := *flagDest
	err = os.MkdirAll(destDir, 0755)
	if err != nil {
		exitWithError(err,2)
	}
	sourceFile :=path.Join(destDir, nameSourceFile)
	err = rename(file.Name(),sourceFile)
	if err != nil {
		exitWithError(err,3)
	}
	sourceVersionInfoFilePath :=path.Join(destDir, nameVersionInfoFile)
	manifestInfoFilePath :=path.Join(destDir, nameManifestFile)
	goGetDependencies([]string{"github.com/segator/jbinary"})
	extension := "bin"
	if strings.Compare(*flagPlatform,"windows")==0 {
		fmt.Println("Go Generate version info")
		_,err =generateVersionInfoFile(sourceVersionInfoFilePath)
		if err != nil {
			exitWithError(err,5)
		}
		fmt.Println("Go Generate windows manifest file")
		_,err =generateManifestFile(manifestInfoFilePath)
		if err != nil {
			exitWithError(err,5)
		}
		goGetDependencies([]string{"github.com/josephspurrier/goversioninfo/cmd/goversioninfo"})
		goversionInfo:=exec.Command("goversioninfo","-manifest",manifestInfoFilePath,"-description",*flagWinDescription,"-copyright",*flagWinCopyright,"-company",*flagWinCompany,"-icon",*flagWinIconPath,
			"-product-name",*flagWinProductName,"-product-version",*flagWinProductVersion,"-ver-major",*flagWinMajorVersion,"-ver-minor",*flagWinMinorVersion,"-ver-patch",*flagWinPatchVersion,
			"-trademark",*flagWinCompany)
		goversionInfo.Env=os.Environ()
		goversionInfo.Dir=destDir
		goversionInfo.Stdout = os.Stdout
		goversionInfo.Stderr = os.Stderr
		goversionInfo.Start()
		if err := goversionInfo.Wait(); err != nil {
			exitWithError(err,6)
		}
		extension="exe"
	}

	fmt.Printf("Building Jre embeded application OS:%s  ARCH:%s  FILENAME:%s\n",*flagPlatform,*flagArchitecture,filepath.Join(destDir,namePackage+"."+extension))
	//"-ldflags","-H=windowsgui",
	cmd := exec.Command("go","build","-o",namePackage+"."+extension)
	cmd.Env=append(os.Environ(),"GOOS="+*flagPlatform,"GOARCH="+*flagArchitecture)
	cmd.Dir=destDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	var exitCode = 0
	if err := cmd.Wait(); err != nil {
		exitError := err.(*exec.ExitError)
		ws := exitError.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
	}
	fmt.Printf("deleting Working Folder:%s\n",tempWorkFolder)
	os.Remove(sourceFile)
	os.Remove(path.Join(destDir, "resource.syso"))
	os.Remove(sourceVersionInfoFilePath)
	os.Remove(manifestInfoFilePath)
	os.RemoveAll(tempWorkFolder)
	os.Exit(exitCode)
}
func generateManifestFile(srcPath string) (file *os.File, err error) {
	f, err := os.Create(srcPath)
	if err != nil {
		log.Fatal("Cannot create file", err)
	}
	defer f.Close()
	fmt.Fprintf(f, `<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity
    type="win32"
    name="%s"
    version="%s.%s.%s.%s"    
    processorArchitecture="IA64"/>
 <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
   <security>
     <requestedPrivileges>
       <requestedExecutionLevel
         level="%s"
         uiAccess="false"/>
       </requestedPrivileges>
   </security>
 </trustInfo>
</assembly>`,*flagWinProductName,*flagWinMajorVersion,*flagWinMinorVersion,*flagWinPatchVersion,*flagWinBuildVersion,*flagWinExecutionLevel)
	return f,err
}

func goGetDependencies(dependencies []string) {
	for _, dependency := range dependencies {
		goget:=exec.Command("go","get",dependency)
		goget.Env=os.Environ()
		goget.Stdout = os.Stdout
		goget.Stderr = os.Stderr
		goget.Start()
		if err := goget.Wait(); err != nil {
			exitWithError(err,7)
		}
	}
}
/*
func test(zip string) {
	jvmArguments:=[]string{}
	staticJavaAppArguments:=[]string{"--debug"}
	javaAppArguments :=staticJavaAppArguments
	debugPort := 21500
	defaultExecutionBehaviour:="gui"
	forceConsoleBehaviourArgs:=[]string{"-console"}
	var function=func(show bool)  {
		var getWin = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow")
		var showWin = syscall.NewLazyDLL("user32.dll").NewProc("ShowWindow")
		hwnd, _, _ := getWin.Call()
		if hwnd == 0 {
			return
		}
		if show {
			var SW_RESTORE uintptr = 9
			showWin.Call(hwnd, SW_RESTORE)
		} else {
			var SW_HIDE uintptr = 0
			showWin.Call(hwnd, SW_HIDE)
		}
	}
	loader.ExecuteJavaApplication(defaultExecutionBehaviour,forceConsoleBehaviourArgs,jvmArguments,javaAppArguments,debugPort,&zip,function)
}*/
// rename tries to os.Rename, but fall backs to copying from src
// to dest and unlink the source if os.Rename fails.
func rename(src, dest string) error {
	// Try to rename generated source.
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	// If the rename failed (might do so due to temporary file residing on a
	// different device), try to copy byte by byte.
	rc, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		rc.Close()
		os.Remove(src) // ignore the error, source is in tmp.
	}()

	if _, err = os.Stat(dest); !os.IsNotExist(err) {
		if err = os.Remove(dest); err != nil {
			return fmt.Errorf("file %q could not be deleted", dest)
		}

	}

	wc, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer wc.Close()

	if _, err = io.Copy(wc, rc); err != nil {
		// Delete remains of failed copy attempt.
		os.Remove(dest)
	}
	return err
}
func generateVersionInfoFile(srcPath string) (file *os.File, err error) {
	f, err := os.Create(srcPath)
	if err != nil {
		log.Fatal("Cannot create file", err)
	}
	defer f.Close()

	fmt.Fprintf(f, `{
  "FixedFileInfo": {
    "FileFlagsMask": "3f",
    "FileFlags ": "00",
    "FileOS": "040004",
    "FileType": "01",
    "FileSubType": "00"
  },
  "VarFileInfo": {
    "Translation": {
      "LangID": "0409",
      "CharsetID": "04B0"
    }
  }
}`)
	return f,err
}


func generateSource(jvmArguments string,appArguments string,srcPath string) (file *os.File, err error) {
	var (
		buffer    bytes.Buffer
		zipWriter io.Writer
	)

	zipWriter = &buffer
	f, err := ioutil.TempFile("", namePackage)
	if err != nil {
		return
	}

	zipWriter = io.MultiWriter(zipWriter, f)
	defer f.Close()

	w := zip.NewWriter(zipWriter)
	if err = filepath.Walk(srcPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Ignore directories and hidden files.
		// No entry is needed for directories in a zip file.
		// Each file is represented with a path, no directory
		// entities are required to build the hierarchy.
		if fi.IsDir() || strings.HasPrefix(fi.Name(), ".") {
			return nil
		}
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		fHeader, err := zip.FileInfoHeader(fi)
		if err != nil {
			return err
		}
		if *flagNoMtime {
			// Always use the same modification time so that
			// the output is deterministic with respect to the file contents.
			fHeader.SetModTime(mtimeDate)
		}
		fHeader.Name = filepath.ToSlash(relPath)
		if !*flagNoCompress {
			fHeader.Method = zip.Deflate
		}
		f, err := w.CreateHeader(fHeader)
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		return err
	}); err != nil {
		return
	}
	if err = w.Close(); err != nil {
		return
	}

	jvmArgumentsString := generateCodeStringArray(jvmArguments)
	javaArgumentsString := generateCodeStringArray(appArguments)
	forceConsoleBehaviourArgsString := generateCodeStringArray(*flagWinExecutionBehaviourConsoleArgs)
	windowsFunction :=""
	windowsImports:=""
	if(*flagPlatform == "windows") {
		windowsImports=`"syscall"`
		windowsFunction =`	
		var getWin = syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow")
		var showWin = syscall.NewLazyDLL("user32.dll").NewProc("ShowWindow")
		hwnd, _, _ := getWin.Call()
		if hwnd == 0 {
			return
		}
		if show {
			var SW_RESTORE uintptr = 9
			showWin.Call(hwnd, SW_RESTORE)
		} else {
			var SW_HIDE uintptr = 0
			showWin.Call(hwnd, SW_HIDE)
		}
	`

	}

	//test(string(buffer.Bytes()))
	var qb bytes.Buffer
	fmt.Fprintf(&qb, `// Code generated by jBinary. DO NOT EDIT.
package main

import (
	"github.com/segator/jbinary/loader"
	"os"
    %s
)

func main() {
    jvmArguments:=[]string{%s}
    staticJavaAppArguments:=[]string{%s}
    defaultExecutionBehaviour:="%s"
	forceConsoleBehaviourArgs:=[]string{%s}
	javaAppArguments :=append(staticJavaAppArguments,os.Args[1:]...)
    debugPort := %d
	var function=func(show bool)  {
		%s
	}
	data := "`,windowsImports,jvmArgumentsString,javaArgumentsString,*flagWinExecutionBehaviour,forceConsoleBehaviourArgsString,*flagDebugPort,windowsFunction)
	FprintZipData(&qb, buffer.Bytes())
	fmt.Fprint(&qb, `"
	loader.ExecuteJavaApplication(defaultExecutionBehaviour,forceConsoleBehaviourArgs,jvmArguments,javaAppArguments,debugPort,&data,function)
}
`)
	if err = ioutil.WriteFile(f.Name(), qb.Bytes(), 0644); err != nil {
		return
	}
	return f, nil
}


func generateCodeStringArray(parameters string) string {
	parameters = strings.TrimSpace(parameters)
	if parameters != "" {
		parametersSlice := strings.Split(parameters,";")
		var templStr = `{{range $i, $e := $}}{{if $i}},{{end}}"{{$e}}"{{end}}`
		var tpl bytes.Buffer
		t := template.Must(template.New("splitParameters").Parse(templStr))
		t.Execute(&tpl, parametersSlice)
		return tpl.String()
	}else {
		return ""
	}

}

// FprintZipData converts zip binary contents to a string literal.
func FprintZipData(dest *bytes.Buffer, zipData []byte) {
	for _, b := range zipData {
		if b == '\n' {
			dest.WriteString(`\n`)
			continue
		}
		if b == '\\' {
			dest.WriteString(`\\`)
			continue
		}
		if b == '"' {
			dest.WriteString(`\"`)
			continue
		}
		if (b >= 32 && b <= 126) || b == '\t' {
			dest.WriteByte(b)
			continue
		}
		fmt.Fprintf(dest, "\\x%02x", b)
	}
}

// Prints out the error message and exists with a non-success signal.
func exitWithError(err error,exitCode int) {

	fmt.Println(err)
	os.Exit(exitCode)
}

func downloadJRE(workDir string){
	var jreTarPath = filepath.Join(workDir,"jre.tar.gz")
	var ossystem="linux"
	var arch="32"
	if *flagArchitecture == "amd64" {
		arch="64"
	}
	if *flagPlatform == "windows" {
		ossystem="win"
	}else{
		arch=""
	}
	var jreURL =strings.Replace(*jreDownloadURL,"{javaType}",*flagJavaType,-1)
	jreURL=strings.Replace(jreURL,"{javaVersion}",*flagJreVersio,-1)
	jreURL=strings.Replace(jreURL,"{serverURL}",*flagServerURL,-1)
	jreURL=strings.Replace(jreURL,"{platform}",ossystem,-1)
	jreURL=strings.Replace(jreURL,"{architecture}",arch,-1)
	DownloadFile(jreTarPath,jreURL)
	fmt.Printf("Extracting JRE version:%s  architecture:%s\n",ossystem,arch)
	ExtractTarGz(jreTarPath,workDir)
	os.Remove(jreTarPath)


}

func DownloadFile(filepath string, url string) error {
	fmt.Printf("Downloading %s\n",url)
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if resp.StatusCode == 404 || resp.StatusCode == 409 {
		exitWithError(errors.New(strings.Replace("URL Repository JRE not found: {url}","{url}",url,-1)),4)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
func ExtractTarGz(srcTarGzPath string,destination string) {
	file, err := os.Open(srcTarGzPath)
	gzipStream :=bufio.NewReader(file)
	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		log.Fatal("ExtractTarGz: NewReader failed")
	}

	tarReader := tar.NewReader(uncompressedStream)

	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatalf("ExtractTarGz: Next() failed: %s", err.Error())
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(filepath.Join(destination,header.Name), 0755); err != nil {
				log.Fatalf("ExtractTarGz: Mkdir() failed: %s", err.Error())
			}
		case tar.TypeReg:
			outFile, err := os.Create(filepath.Join(destination,header.Name))
			if err != nil {
				log.Fatalf("ExtractTarGz: Create() failed: %s", err.Error())
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				log.Fatalf("ExtractTarGz: Copy() failed: %s", err.Error())
			}
		case tar.TypeLink,tar.TypeSymlink,tar.TypeChar,tar.TypeBlock,tar.TypeFifo:
			log.Printf("Ignoring tar object:%s file:%s",string(header.Typeflag),header.Name)
		default:
			log.Fatalf(
				"ExtractTarGz: uknown type: %s in %s",
				string(header.Typeflag),
				header.Name)
		}
	}
	file.Close()
}

func Copy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}