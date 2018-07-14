# JBinary
JBinary Embed JRE inside a windows/linux binary to allow you to distribute your
java application without be worried about install java on client machines.
A single windows linux binary is shiped out of the box.

```bash
  -architecture string
        Building Architecture (amd64|386) (default "amd64")
  -build string
        The destination path of the generated package. (default ".")
  -ignore-modtime
        Ignore modification times on files.
  -jar string
        Path of the java application (default "application.jar")
  -jre-version string
        The destination path of the generated package. (default "1.8.0_131")
  -no-compress
        Do not use compression to shrink the files.
  -output-name string
        Name of the generated package (default "application")
  -platform string
        Operating system(linux|windows) (default "windows")
```

## Install
### Linux
```
wget https://github.com/segator/jbinary/releases/download/0.0.2/linux_amd64_jbinary_0.0.2 -O /usr/bin/JBinary && chmod 777 /usr/bin/JBinary
```

### Windows
Execute as admin
```
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest -Uri "https://github.com/segator/jbinary/releases/download/0.0.2/windows_amd64_jbinary_0.0.2.exe" -OutFile "C:\Windows\System32\JBinary.exe" 
```
## Example
```bash
JBinary -platform windows -output-name myapp -jar myjavaapp.jar  -build /root/provas/build

#A myjavaapp.exe is generated on /root/provas/build
JBinary -platform linux -output-name myapp -jar myjavaapp.jar  -build /root/provas/build
#A myjavaapp.bin is generated on /root/provas/build
```