package main

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"flag"
	//"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/prometheus/log"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	pb "github.com/federicobaldo/go-kura/kuradatatypes"
	"strings"
)

func getPayload(payloadBytes []byte) ([]byte, error) {
	log.Debugf("Maybe this is compressed...")
	gzipReader, err := gzip.NewReader(bytes.NewReader(payloadBytes))
	if err != nil {
		log.Debugf("Not gzipped: %v", err) //Not a gzip payload
		return payloadBytes, nil
	}
	bytesArray, err := ioutil.ReadAll(gzipReader)
	log.Debugf("Read %v bytes.", len(bytesArray))
	if err != nil {
		log.Infof("Maybe it is not compressed...")
		bytesArray = payloadBytes
	}

	return bytesArray, nil
}

func onMessageReceived(client MQTT.Client, message MQTT.Message) {
	bytesArray, err := getPayload(message.Payload())
	if err != nil {
		log.Fatal("Unable to unmarshal payload")
	}
	kuraPayload := &pb.KuraPayload{}
	err = proto.Unmarshal(bytesArray, kuraPayload)

	if err != nil {
		log.Errorf("%v", err)
		log.Errorf("Not a valid Kura message: %s\nMessage: %s\n", message.Topic(), message.Payload())
		return
	}

	//marshaler := &jsonpb.Marshaler{}
	//jsonString, _ := marshaler.MarshalToString(kuraPayload)

	log.Infof("Sensor ID: %s", message.Topic()[strings.LastIndex(message.Topic(), "/") + 1: ])

	for _, metric := range kuraPayload.Metric {
		switch metric.GetType() {
		case pb.KuraPayload_KuraMetric_INT32:
			log.Infof("\t%s (%s): %v", metric.GetName(), metric.GetType(), metric.GetIntValue())
		case pb.KuraPayload_KuraMetric_INT64:
			log.Infof("\t%s (%s): %v", metric.GetName(), metric.GetType(), metric.GetLongValue())
		case pb.KuraPayload_KuraMetric_BOOL:
			log.Infof("\t%s (%s): %v", metric.GetName(), metric.GetType(), metric.GetBoolValue())
		case pb.KuraPayload_KuraMetric_DOUBLE:
			log.Infof("\t%s (%s): %v", metric.GetName(), metric.GetType(), metric.GetDoubleValue())
		case pb.KuraPayload_KuraMetric_FLOAT:
			log.Infof("\t%s (%s): %v", metric.GetName(), metric.GetType(), metric.GetFloatValue())
		case pb.KuraPayload_KuraMetric_BYTES:
			log.Infof("\t%s (%s): %v", metric.GetName(), metric.GetType(), metric.GetBoolValue())
		case pb.KuraPayload_KuraMetric_STRING:
			log.Infof("\t%s (%s): %s", metric.GetName(), metric.GetType(), metric.GetStringValue())
		default:
			log.Infof("\t%s (%s): %s", metric.GetName(), metric.GetType(), metric.GetStringValue())
		}
	}
}

var i int

func main() {
	//MQTT.DEBUG = log.New(os.Stdout, "", 0)
	//MQTT.ERROR = log.New(os.Stdout, "", 0)
	c := make(chan os.Signal, 1)
	i = 0
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Infof("signal received, exiting")
		os.Exit(0)
	}()

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
	connOpts.OnConnect = func(c MQTT.Client) {
		if token := c.Subscribe(*topic, byte(*qos), onMessageReceived); token.Wait() && token.Error() != nil {
			log.Fatalf("%v", token.Error())
		}
	}

	client := MQTT.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("%v", token.Error())
	} else {
		log.Infof("Connected to %s\n", *server)
	}

	for {
		time.Sleep(1 * time.Second)
	}
}
