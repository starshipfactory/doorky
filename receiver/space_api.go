package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/starshipfactory/doorky"
)

// SpaceAPI provides a read-only HTTP handler for the Hackerspace API
// (see http://spaceapi.net/).
type SpaceAPI struct {
	ts   *doorky.Timeseries
	conf *doorky.DoorkyConfig
}

// NewSpaceAPI creates a new hackerspace API object which can be served over
// HTTP to requestors. It will query the timeseries store specified as "ts".
func NewSpaceAPI(ts *doorky.Timeseries, conf *doorky.DoorkyConfig) *SpaceAPI {
	return &SpaceAPI{
		ts:   ts,
		conf: conf,
	}
}

// Compile all information exported through the hackerspace API and send it
// back to the requestor.
func (a *SpaceAPI) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var msg = proto.Clone(a.conf.SpaceapiMd)
	var md *doorky.SpaceAPIMetadata
	var door *doorky.DoorSecret
	var out []byte
	var err error
	var ok bool

	md, ok = msg.(*doorky.SpaceAPIMetadata)
	if !ok {
		log.Print("Error: message is not of type SpaceAPIMetadata")
		http.Error(rw, http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
	}

	for _, door = range a.conf.Secret {
		var lockInfo = new(doorky.SpaceAPIDoorLockSensor)
		var name = door.GetName()
		var ts time.Time
		var locked bool

		ts, locked, err = a.ts.LastValue(name)
		if err != nil {
			log.Print("Error fetching door status for ", name, ": ", err)
			continue
		}

		lockInfo.Value = proto.Bool(locked)
		lockInfo.Location = proto.String(door.GetLocation())
		lockInfo.Name = proto.String(name)
		lockInfo.Description = proto.String("Last update: " + ts.String())

		if md.Sensors == nil {
			md.Sensors = new(doorky.SpaceAPISensors)
		}
		md.Sensors.DoorLocked = append(md.Sensors.DoorLocked, lockInfo)

		if a.conf.PrimaryDoor != nil && a.conf.GetPrimaryDoor() == name {
			md.State.Open = proto.Bool(!locked)
			md.State.Lastchange = proto.Int64(ts.Unix())
		}
	}

	out, err = json.MarshalIndent(md, "    ", "    ")
	if err != nil {
		log.Print("Error marshalling JSON: ", err)
		http.Error(rw, http.StatusText(http.StatusInternalServerError)+": "+
			"Error marshalling response: "+err.Error(),
			http.StatusInternalServerError)
	}

	_, err = rw.Write(out)
	if err != nil {
		log.Print("Error writing response: ", err)
	}
}
