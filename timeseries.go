package doorky

import (
	"database/cassandra"
	"encoding/binary"
	"time"
)

// Timeseries is an object to access the stream of data points from
type Timeseries struct {
	db *cassandra.RetryCassandraClient
}

// NewTimeseries creates a new connection to the timeseries database.
func NewTimeseries(host string, timeout time.Duration,
	credentials []*CassandraCredentials, keyspace string) (
	*Timeseries, error) {
	var cred *CassandraCredentials
	var creds map[string]string
	var db *cassandra.RetryCassandraClient
	var err error

	creds = make(map[string]string)

	for _, cred = range credentials {
		creds[cred.GetKey()] = cred.GetValue()
	}

	db, err = cassandra.NewRetryCassandraClientTimeout(host, timeout)
	if err != nil {
		return nil, err
	}

	if len(creds) > 0 {
		var ar = cassandra.NewAuthenticationRequest()
		ar.Credentials = creds

		err = db.Login(ar)
		if err != nil {
			return nil, err
		}
	}

	err = db.SetKeyspace(keyspace)
	if err != nil {
		return nil, err
	}

	return &Timeseries{
		db: db,
	}, err
}

// Insert a value into the time series. The value should record that the door
// status at the time "stamp" was "value" (true for open, false for closed).
func (ts *Timeseries) Insert(door string, stamp time.Time, value bool) error {
	var cp = cassandra.NewColumnParent()
	var col = cassandra.NewColumn()
	var timestamp = stamp.UnixNano() / 1000
	var tsdata []byte = make([]byte, 8)

	binary.BigEndian.PutUint64(tsdata, uint64(-timestamp))

	cp.ColumnFamily = "timeseries"
	col.Name = []byte(door)
	col.Timestamp = &timestamp

	if value {
		col.Value = []byte{1}
	} else {
		col.Value = []byte{0}
	}

	return ts.db.Insert(tsdata, cp, col, cassandra.ConsistencyLevel_QUORUM)
}
