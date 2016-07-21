package main

import (
	"crypto/sha256"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/starshipfactory/doorky"
)

func main() {
	var secrets = make(map[string][]byte)
	var secret *doorky.DoorSecret
	var ts *doorky.Timeseries

	var wapi *WriteAPI
	var sapi *SpaceAPI

	var config doorky.DoorkyConfig
	var configPath string
	var configData []byte
	var bindAddress string
	var err error

	flag.StringVar(&configPath, "config", "Path to the configuration file", "")
	flag.StringVar(&bindAddress, "bind",
		"host:port pair to bind the HTTP server to", ":8080")
	flag.Parse()

	configData, err = ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatal("Error reading config file ", configPath, ": ", err)
	}

	err = proto.UnmarshalText(string(configData), &config)
	if err != nil {
		log.Fatal("Error reading config file: ", err)
	}

	for _, secret = range config.GetSecret() {
		var secretInfo = sha256.Sum256([]byte(secret.GetSecret()))
		secrets[secret.GetName()] = secretInfo[:]
	}

	ts, err = doorky.NewTimeseries(config.GetDbServer(),
		time.Duration(config.GetDbTimeout())*time.Millisecond,
		config.GetDbCredentials(), config.GetKeyspace())
	if err != nil {
		log.Fatal("Error connecting to timeseries database: ", err)
	}

	wapi = NewWriteAPI(ts, secrets)
	http.Handle("/api/doorstatus", wapi)

	if config.SpaceapiMd != nil {
		sapi = NewSpaceAPI(ts, &config)
		http.Handle("/api/spaceapi", sapi)
	}
	err = http.ListenAndServe(bindAddress, nil)
	if err != nil {
		log.Fatal("Error listening to ", bindAddress, ": ", err)
	}
}
