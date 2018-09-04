/*
Copyright 2018 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"flag"
	"fmt"
	"strings"

	//"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	ui "github.com/gizak/termui"
	kpb "github.com/redhat-iot/kurasowa/kuradatatypes"
)

var (
	commitHash 	string
	timestamp  	string
	gitTag     	= "SNAPSHOT"
	gui			bool
	metric1		= make(map[string][]float64)
)

type Metric struct {
	machine string
	payload *kpb.KuraPayload
}

func getPayload(payloadBytes []byte) ([]byte, error) {
	log.Debugf("Maybe this is compressed...")
	gzipReader, err := gzip.NewReader(bytes.NewReader(payloadBytes))
	if err != nil {
		log.Infof("Not gzipped: %v", err) //Not a gzip payload
		return payloadBytes, nil
	}
	bytesArray, err := ioutil.ReadAll(gzipReader)
	log.Debugf("Read %v bytes.", len(bytesArray))
	if err != nil {
		log.Infof("Maybe it is not compressed after all...")
		bytesArray = payloadBytes
	}
	log.Debugf("gzipped Kura Payload...")
	return bytesArray, nil
}

func onMessageReceived(client MQTT.Client, message MQTT.Message) {
	bytesArray, err := getPayload(message.Payload())
	ctxLogger := log.WithFields(log.Fields{
		//"id": message.MessageID(),
		"topic": message.Topic(),
	})

	if err != nil {
		ctxLogger.Fatal("Unable to unmarshal payload")
	}
	kuraPayload := &kpb.KuraPayload{}
	err = proto.Unmarshal(bytesArray, kuraPayload)

	if err != nil {
		ctxLogger.Errorf("%v", err)
		ctxLogger.Errorf("Not a valid Kura message: %s\nMessage: %s\n", message.Topic(), message.Payload())
		return
	}

	// TODO: Reference for output to JSON
	//marshaler := &jsonpb.Marshaler{}
	//jsonString, _ := marshaler.MarshalToString(kuraPayload)

	//tm := time.Unix(0, kuraPayload.GetTimestamp() * int64(time.Millisecond))
	//ctxLogger.Infof("Full Topic: %s", message.Topic())
	//ctxLogger.Infof("Sensor ID: %s - Timestamp: %s", message.Topic()[strings.LastIndex(message.Topic(), "/") + 1: ], tm.Local())

	fields := log.Fields{}
	for _, metric := range kuraPayload.Metric {
		switch metric.GetType() {
		case kpb.KuraPayload_KuraMetric_INT32:
			fields[metric.GetName()] = metric.GetIntValue()
		case kpb.KuraPayload_KuraMetric_INT64:
			fields[metric.GetName()] = metric.GetLongValue()
		case kpb.KuraPayload_KuraMetric_BOOL:
			fields[metric.GetName()] = metric.GetBoolValue()
		case kpb.KuraPayload_KuraMetric_DOUBLE:
			fields[metric.GetName()] = metric.GetDoubleValue()
			/*
			if (metric.GetName() == "timestamp") {
				fields["metric_timestamp"] = time.Unix(0, int64(time.Millisecond) * int64(metric.GetDoubleValue()))
			}
			*/
		case kpb.KuraPayload_KuraMetric_FLOAT:
			fields[metric.GetName()] = metric.GetFloatValue()
		case kpb.KuraPayload_KuraMetric_BYTES:
			fields[metric.GetName()] = metric.GetBytesValue()
		case kpb.KuraPayload_KuraMetric_STRING:
			fields[metric.GetName()] = metric.GetStringValue()
		default:
		}
	}
	fields["payload_timestamp"] = time.Unix(0, kuraPayload.GetTimestamp() * int64(time.Millisecond))



	if gui {
		ui.SendCustomEvt("/data/update", &Metric{message.Topic(), kuraPayload})
	} else {
		ctxLogger.WithFields(fields).Infof("Kura Metric Payload:")
	}

}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Kurasowa %s : Build Time %s : Commit %s\n", gitTag, timestamp, commitHash )
		flag.PrintDefaults()
	}

	hostname, _ := os.Hostname()

	server := flag.String("server", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	topic := flag.String("topic", "#", "Topic to subscribe to")
	qos := flag.Int("qos", 0, "The QoS to subscribe to messages at")
	clientid := flag.String("clientid", hostname+strconv.Itoa(time.Now().Second()), "A clientid for the connection")
	username := flag.String("username", "", "A username to authenticate to the MQTT server")
	password := flag.String("password", "", "Password to match username")
	flag.BoolVar(&gui, "gui", false, "Show GUI")
	flag.Parse()

	fmt.Printf("GUI value %v", gui)

	//cer, err := tls.LoadX509KeyPair("/path/to/cert.pem")
	//if err != nil {
	//	panic(err)
	//}

	//check(err)

	//MQTT.DEBUG = log.New(os.Stdout, "", 0)
	//MQTT.ERROR = log.New(os.Stdout, "", 0)

	CheckGUI:
	if gui {

		err := ui.Init()
		if err != nil {
			log.Warnf("Unable to run GUI mode: %v\n", err)
			gui = false
			goto CheckGUI
		}
		defer ui.Close()

		ui.SetOutputMode(ui.Output256)
		p := ui.NewPar("Press 'q' to Quit")
		p.Height = 3
		//p.Width = 50
		p.TextFgColor = ui.ColorWhite
		p.BorderLabel = "Text Box"
		p.BorderFg = ui.ColorCyan
		p.Handle("/timer/1s", func(e ui.Event) {
			cnt := e.Data.(ui.EvtTimer)
			if cnt.Count%2 == 0 {
				p.TextFgColor = ui.ColorRed
			} else {
				p.TextFgColor = ui.ColorWhite
			}
		})

/*		sinps := (func() []float64 {
			n := 220
			ps := make([]float64, n)
			for i := range ps {
				ps[i] = 1 + math.Sin(float64(i)/5)
			}
			return ps
		})()
*/
/*
		gs := make(map[string]*ui.Gauge)
		for _, machine := range [3]string{0: "machine-1", 1: "machine-2", 2: "machine-3"} {
			gs[machine] = ui.NewGauge()
			//gs[i].LabelAlign = ui.AlignCenter
			gs[machine].Height = 2
			gs[machine].Border = false
			//gs[i].Percent = i * 10
			gs[machine].PaddingBottom = 1
			gs[machine].BarColor = ui.ColorBlue
		}
*/
		lc := make(map[string]*ui.LineChart)
		for _, machine := range [3]string{0: "machine-1", 1: "machine-2", 2: "machine-3"} {

			lc[machine] = ui.NewLineChart()
			lc[machine].BorderLabel = machine
			//lc0.Mode = "dot"
			//lc0.Data = metric1
			lc[machine].DataLabels = []string{"machine-1"} //, "machine-2", "machine-3"}
			//lc0.Width = 60
			lc[machine].Height = 16
			lc[machine].X = 0
			lc[machine].Y = 0
			lc[machine].AxesColor = ui.ColorWhite
			lc[machine].LineColor["temp"] = ui.ColorCyan // | ui.AttrBold
			lc[machine].LineColor["voltage"] = ui.ColorYellow // | ui.AttrBold
			lc[machine].LineColor["speed"] = ui.ColorMagenta // | ui.AttrBold
			lc[machine].BorderFg = ui.ColorWhite

		}

		ui.Body.AddRows(
			ui.NewRow(
				ui.NewCol(12, 0, p)),
		)

		for _, v := range lc {
			ui.Body.AddRows(
				ui.NewRow(
					ui.NewCol(12, 0, v)),
			)

		}
/*		ui.Body.AddRows(
			ui.NewRow(
				ui.NewCol(12, 0, p)),
			ui.NewRow(
				ui.NewCol(12, 0, lc0)),
			ui.NewRow(
				ui.NewCol(3, 0, gs["machine-1"], gs["machine-2"], gs["machine-3"]),
				//ui.NewCol(6, 0, widget1)),
			),
			ui.NewRow(
				ui.NewCol(6, 0, lc1),
				ui.NewCol(6, 0, lc2),
		))
*/

		// calculate layout
		ui.Body.Align()

		ui.Render(ui.Body)


		//ui.Render(lc0, lc1, lc2)
		ui.Handle("/sys/kbd/q", func(ui.Event) {
			ui.StopLoop()
		})
		ui.Handle("/sys/wnd/resize", func(e ui.Event) {
			ui.Body.Width = ui.TermWidth()
			ui.Body.Align()
			ui.Clear()
			ui.Render(ui.Body)
		})
		//draw := func(t int) {
		//	lc0.Data = append(lc0.Data, float64(t))
		//	ui.Render(lc0, lc1, lc2)
		//}
		ui.Handle("/data/update", func(evt ui.Event) {
			metricIn := evt.Data.(*Metric)
			kp := metricIn.payload

			topicElements := strings.Split(metricIn.machine, "/")
			machine := topicElements[len(topicElements)-1]

/*			if val, ok := lc0.Data["foo"]; !ok {
				lc0.Data[machine]. //Do something
			}
*/
			for _, metric := range kp.Metric {
				//fmt.Printf("Metric Name: %s of type %s\n", metric.GetName(), metric.GetType())

				switch metric.GetType() {
				case kpb.KuraPayload_KuraMetric_INT32:
					//fields[metric.GetName()] = metric.GetIntValue()
				case kpb.KuraPayload_KuraMetric_INT64:
					//fields[metric.GetName()] = metric.GetLongValue()
				case kpb.KuraPayload_KuraMetric_BOOL:
					//fields[metric.GetName()] = metric.GetBoolValue()
				case kpb.KuraPayload_KuraMetric_DOUBLE:
					if (metric.GetName() == "temp") {
						//gs[machine].Percent = int(((metric.GetDoubleValue() - 82) * 100) / (86 - 82))
						//gs[machine].Label = fmt.Sprintf("%q is at %3.3f C", machine, metric.GetDoubleValue())
						lc[machine].DataLabels[0] = metric.GetName()
						if len(metric1) < (lc[machine].InnerWidth() * 2) - (lc[machine].PaddingLeft + lc[machine].PaddingRight + 10) {
							metric1[machine] = append(metric1[machine], metric.GetDoubleValue())
						} else {
							metric1[machine] = append(metric1[machine][1:], metric.GetDoubleValue())
						}

						lc[machine].Data[machine] = metric1[machine] //[len(metric1)-30:]  //["metric_timestamp"] = time.Unix(0, int64(time.Millisecond) * int64(metric.GetDoubleValue()))
					}

					//fields[metric.GetName()] = metric.GetDoubleValue()
					/*
					if (metric.GetName() == "timestamp") {
						fields["metric_timestamp"] = time.Unix(0, int64(time.Millisecond) * int64(metric.GetDoubleValue()))
					}
					*/
				case kpb.KuraPayload_KuraMetric_FLOAT:
					//fields[metric.GetName()] = metric.GetFloatValue()
				case kpb.KuraPayload_KuraMetric_BYTES:
					//fields[metric.GetName()] = metric.GetBytesValue()
				case kpb.KuraPayload_KuraMetric_STRING:
					//fields[metric.GetName()] = metric.GetStringValue()
				default:
				}
			}
			ui.Render(lc[machine])
		})
		//ui.Handle("/timer/1s", func(e ui.Event) {
		//	t := e.Data.(ui.EvtTimer)
		//	draw(int(t.Count))
		//})

	}

	//println("At step 1")

	connOpts := &MQTT.ClientOptions{
		ClientID:             *clientid,
		CleanSession:         true,
		Username:             *username,
		Password:             *password,
		MaxReconnectInterval: 1 * time.Second,
		KeepAlive:            0, //30 * time.Second,
		TLSConfig:            tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert},
		AutoReconnect:		  true,
		ConnectTimeout: 	  5 * time.Second,

	}
	connOpts.AddBroker(*server)
	connOpts.SetConnectionLostHandler(func(c MQTT.Client, err error) {
		log.Warnf("Connection has been lost: %s", err)
	})
	connOpts.SetOnConnectHandler(func(c MQTT.Client) {
		log.Debugf("Connected...")
		if token := c.Subscribe(*topic, byte(*qos), onMessageReceived); token.Wait() && token.Error() != nil {
			log.Fatalf("%v", token.Error())
		}
	})

	client := MQTT.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("%v", token.Error())
	} else {
		log.Debugf("Connected to %s\n", *server)
	}

	if gui {
		ui.Loop()

	} else {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		log.Infof("signal received, exiting")
		client.Disconnect(5000)
		os.Exit(0)
	}


}

func catch(err error) {
	if err != nil {
		log.Fatalf("%v", err)
		panic(err)
	}
}