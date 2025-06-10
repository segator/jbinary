# JBinary
JBinary Embed JRE inside a windows/linux binary to allow you to distribute your
java application without be worried about install java on client machines.
A single windows linux binary is shiped out of the box.

```bash
  -app-arguments string
        App Arguments
  -architecture string
        Building Architecture (amd64|386) (default "amd64")
  -build string
        The destination path of the generated package. (default ".")
  -ignore-modtime
        Ignore modification times on files.
  -jar string
        Path of the java application (default "application.jar")
  -java-download-link string
        Link where to download java distribution format:{serverURL}/com/oracle/java/{javaType}/{javaVersion}/{javaType}-{javaVersion}-{platform}{architecture}.tgz (default "{serverURL}/com/oracle/java/{javaType}/{javaVersion}/{javaType}-{javaVersion}-{platform}{architecture}.tgz")
  -java-server-url string
        Server base URL to look for java download (default "https://artifacts.alfresco.com/nexus/content/repositories/public")
  -java-type string
        Java type jre|jdk (default "jre")
  -jre-debug-port int
        Debug port to listen if the generated binary is executed with cli -debug (default 21500)
  -jre-version string
        The destination path of the generated package. (default "1.8.0_131")
  -jvm-arguments string
        JVM Arguments
  -no-compress
        Do not use compression to shrink the files.
  -output-name string
        Name of the generated package (default "application")
  -platform string
        Operating system(linux|windows) (default "windows")
  -win-company string
        Windows Application company name (default "no company")
  -win-copyright string
        Windows Application copyright (default "no copyright")
  -win-description string
        Windows Application description (default "no description")
  -win-execution-behaviour string
        Default behaviour to run app, in gui mode no console is shown but you can't execute by console or capture stdout, (console|gui) default(console) (default "console")
  -win-execution-enable-console-behaviour-args string
        Arguments that will force console mode in case of default behaviour gui, default (-console;-terminal) (default "-console;-terminal")
  -win-icon-path string
        icon path
  -win-invoker string
        Windows Invoker type  asInvoker|requireAdministrator default(asInvoker) (default "asInvoker")
  -win-product-name string
        Windows Application product name (default "product name")
  -win-product-version string
        Windows Application product version (default "1.0.0.0")
  -win-version-major string
        Windows Application Major version (default "1")
  -win-version-minor string
        Windows Application Minor version
  -win-version-patch string
        Windows Application Patch version
```

## Install
### Linux
```
wget https://github.com/segator/jbinary/releases/download/0.0.6/jbinary-0.0.6_linux_amd64 -O /usr/bin/JBinary && chmod 777 /usr/bin/JBinary # x-release-please-version
```

### Windows
Execute as admin
```
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest -Uri "https://github.com/segator/jbinary/releases/download/0.0.6/jbinary-0.0.6-windows_amd64.exe" -OutFile "C:\Windows\System32\JBinary.exe" # x-release-please-version
```
## Example
```bash
JBinary -platform windows -output-name myapp -jar myjavaapp.jar  -build /root/provas/build

#A myjavaapp.exe is generated on /root/provas/build
JBinary -platform linux -output-name myapp -jar myjavaapp.jar  -build /root/provas/build
#A myjavaapp.bin is generated on /root/provas/build
```