package main

import (
	"encoding/json"
	"log"
	"net/http"

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
	var ok bool
	var md *doorky.SpaceAPIMetadata
	var jsonEnc = json.NewEncoder(rw)
	var door *doorky.DoorSecret

	md, ok = msg.(*doorky.SpaceAPIMetadata)
	if !ok {
		log.Print("Error: message is not of type SpaceAPIMetadata")
		http.Error(rw, http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
	}

	for _, door = range a.conf.Secret {
		var lockInfo = new(doorky.SpaceAPIDoorLockSensor)
		var name = door.GetName()
		var locked bool
		var err error

		_, locked, err = a.ts.LastValue(name)
		if err != nil {
			log.Print("Error fetching door status for ", name, ": ", err)
			continue
		}

		lockInfo.Value = proto.Bool(locked)
		lockInfo.Location = proto.String(door.GetLocation())
		lockInfo.Name = proto.String(name)

		if md.Sensors == nil {
			md.Sensors = new(doorky.SpaceAPISensors)
		}
		md.Sensors.DoorLocked = append(md.Sensors.DoorLocked, lockInfo)
	}

	jsonEnc.Encode(md)
}
