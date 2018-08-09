package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"os/exec"
	"syscall"
	"net/http"
	"archive/tar"
	"compress/gzip"
	"log"
	"bufio"
	"github.com/pkg/errors"
)

const (
	nameSourceFile = "data.go"
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
	flagPkg        = flag.String("output-name", "application", "Name of the generated package")
	flagServerURL  = flag.String("java-server-url","https://artifacts.alfresco.com/nexus/content/repositories/public","Server base URL to look for java download")
	flagDefaultDownloadURL="{serverURL}/com/oracle/java/{javaType}/{javaVersion}/{javaType}-{javaVersion}-{platform}{architecture}.tgz"
	jreDownloadURL = flag.String("java-download-link",flagDefaultDownloadURL,"Link where to download java distribution format:"+flagDefaultDownloadURL)
	//https://artifacts.alfresco.com/nexus/content/repositories/public/com/oracle/java/jre/1.8.0_131/jre-1.8.0_131-win64.tgz
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
	file, err := generateSource(tempWorkFolder)
	if err != nil {
		exitWithError(err,-1)
	}

	destDir := *flagDest
	err = os.MkdirAll(destDir, 0755)
	if err != nil {
		exitWithError(err,-2)
	}
	sourceFile :=path.Join(destDir, nameSourceFile)
	err = rename(file.Name(),sourceFile)
	if err != nil {
		exitWithError(err,-3)
	}
	extension := "bin"
	if strings.Compare(*flagPlatform,"windows")==0 {
		extension="exe"
	}
	fmt.Println("Downloading go dependencies")
	goget:=exec.Command("go","get","github.com/segator/jbinary/")
	goget.Env=os.Environ()
	goget.Start()
	goget.Wait()
	fmt.Printf("Building Jre embeded application OS:%s  ARCH:%s  FILENAME:%s\n",*flagPlatform,*flagArchitecture,filepath.Join(destDir,namePackage+"."+extension))
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
	os.RemoveAll(tempWorkFolder)
	os.Exit(exitCode)
}

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

// Walks on the source path and generates source code
// that contains source directory's contents as zip contents.
// Generates source registers generated zip contents data to
// be read by the statik/fs HTTP file system.
func generateSource(srcPath string) (file *os.File, err error) {
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

	// then embed it as a quoted string
	var qb bytes.Buffer
	fmt.Fprintf(&qb, `// Code generated by jBinary. DO NOT EDIT.
package main

import (
	"github.com/segator/jbinary/loader"
	"os"
)

func main() {
	data := "`)
	FprintZipData(&qb, buffer.Bytes())
	fmt.Fprint(&qb, `"
	loader.ExecuteJavaApplication(os.Args[1:],data)
}
`)
	if err = ioutil.WriteFile(f.Name(), qb.Bytes(), 0644); err != nil {
		return
	}
	return f, nil
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
		exitWithError(errors.New(strings.Replace("URL Repository JRE not found: {url}","{url}",url,-1)),-4)
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