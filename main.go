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
	kpb "github.com/redhat-iot/kurasowa/kuradatatypes"
)

var (
	commitHash string
	timestamp  string
	gitTag     = "SNAPSHOT"
)

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

	ctxLogger.WithFields(fields).Infof("Kura Metric Payload:")

}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Kurasowa %s : Build Time %s : Commit %s\n", gitTag, timestamp, commitHash )
		flag.PrintDefaults()
	}

	//MQTT.DEBUG = log.New(os.Stdout, "", 0)
	//MQTT.ERROR = log.New(os.Stdout, "", 0)

	hostname, _ := os.Hostname()

	server := flag.String("server", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	topic := flag.String("topic", "#", "Topic to subscribe to")
	qos := flag.Int("qos", 0, "The QoS to subscribe to messages at")
	clientid := flag.String("clientid", hostname+strconv.Itoa(time.Now().Second()), "A clientid for the connection")
	username := flag.String("username", "", "A username to authenticate to the MQTT server")
	password := flag.String("password", "", "Password to match username")
	flag.Parse()

	connOpts := &MQTT.ClientOptions{
		ClientID:             *clientid,
		CleanSession:         true,
		Username:             *username,
		Password:             *password,
		MaxReconnectInterval: 1 * time.Second,
		KeepAlive:            0, //30 * time.Second,
		TLSConfig:            tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert},
	}
	connOpts.AddBroker(*server)
	connOpts.SetAutoReconnect(true)
	connOpts.SetConnectTimeout(5 * time.Second)
	connOpts.SetConnectionLostHandler(func(c MQTT.Client, err error) {
		log.Warnf("Connection has been lost: %s", err)
	})
	connOpts.SetOnConnectHandler(func(c MQTT.Client) {
		log.Info("Connected...")
		if token := c.Subscribe(*topic, byte(*qos), onMessageReceived); token.Wait() && token.Error() != nil {
			log.Fatalf("%v", token.Error())
		}
	})

	client := MQTT.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("%v", token.Error())
	} else {
		log.Infof("Connected to %s\n", *server)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Infof("signal received, exiting")
	client.Disconnect(5000)
	os.Exit(0)


}
