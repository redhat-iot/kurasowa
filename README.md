# Kurasowa command line util for Eclipse Kura
Currently supports subscribing to Kura / Kapua metrics topics to quickly and easily debug and monitor installations.
Could evolve to support more functions later. Please create issues if you have a requested feature.
## Installation
### Mac OSX
Using homebrew:
```
$ brew tap redhat-iot/homebrew-tap
$ brew install kurasowa
```

### Linux
Download the [the latest release](https://github.com/redhat-iot/kurasowa/releases/latest) in tar.gz format and
move the kurasowa executable to a suitable location in your path.

### Windows
Download the [the latest release](https://github.com/redhat-iot/kurasowa/releases/latest) zip file and
move kurasowa.exe to a suitable location in your path.

## Usage
```
$ kurasowa -username <user> -password <password> -server 'tcp://<mqttBroker>:1883' \
    -topic '<Account>/<Device>/<App>/<Topic>'
```
For example, to connect to the running Industry 4.0 demo instance:
```
$ kurasowa -username 'demo' -password 'DemoUser123!@#' \
    -server 'tcp://broker-redhat-iot.apps.iiot-demo.rhiot.org:31883' \
    -topic 'Red-Hat/+/cloudera-demo/#'
```
