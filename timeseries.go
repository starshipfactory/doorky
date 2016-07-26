package doorky

import (
	"database/cassandra"
	"encoding/binary"
	"errors"
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
	var tsdata = make([]byte, 8)

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

// LastValue looks up the most recent value of the specified time series.
func (ts *Timeseries) LastValue(door string) (
	stamp time.Time, value bool, err error) {
	var cp = cassandra.NewColumnParent()
	var pred = cassandra.NewSlicePredicate()
	var kr = cassandra.NewKeyRange()
	var kss []*cassandra.KeySlice
	var ks *cassandra.KeySlice
	var timestamp uint64

	cp.ColumnFamily = "timeseries"
	pred.ColumnNames = [][]byte{[]byte(door)}
	kr.StartKey = []byte{}
	kr.EndKey = []byte{}
	kr.Count = 1

	kss, err = ts.db.GetRangeSlices(cp, pred, kr, cassandra.ConsistencyLevel_ONE)
	if err != nil {
		return
	}

	for _, ks = range kss {
		var cos *cassandra.ColumnOrSuperColumn
		timestamp = binary.BigEndian.Uint64(ks.Key)
		stamp = time.Unix(int64(timestamp/1000), int64((timestamp%1000)*1000))

		for _, cos = range ks.Columns {
			if cos.Column != nil {
				var col = cos.Column
				if string(col.Name) == door {
					value = (col.Value[0] == 1)
					return
				}
			}
		}
	}

	err = errors.New("No value found for " + door)
	return
}
