package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"expvar"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/starshipfactory/doorky"
)

var invalidForms = expvar.NewInt("invalid-forms")
var invalidDoorName = expvar.NewInt("invalid-door-name")
var unparseableValue = expvar.NewInt("unparseable-value")
var unparseableTimestamp = expvar.NewInt("unparseable-timestamp")
var unknownDoor = expvar.NewInt("unknown-door")
var unparseableHash = expvar.NewInt("unparseable-hash")
var hashVerificationFailed = expvar.NewInt("hash-verification-failed")
var insertError = expvar.NewInt("insert-error")

var doornameRe = regexp.MustCompile("[[:alnum:]]")

// WriteAPI provides an HTTP handler for receiving door status updates.
// Handles signed HTTP GET requests with data to be placed into a time series.
type WriteAPI struct {
	ts      *doorky.Timeseries
	secrets map[string][]byte
}

// NewWriteAPI creates a new Write API around the timeseries store "ts" and
// using the specified "secrets" map to look up door secrets.
func NewWriteAPI(ts *doorky.Timeseries,
	secrets map[string][]byte) *WriteAPI {
	return &WriteAPI{
		ts:      ts,
		secrets: secrets,
	}
}

// They are submitted as HTTP GET requests with the following fields:
//
//  - door is the name of the door which is sending updates,
//  - val is 0 or 1 depending on whether the door is closed or open,
//  - ts is the integer timestamp in seconds since January 1, 1970,
//  - hash is a SHA-256 hash of all of the above, and
//  - iv is the initialization vector used for encrypting the hash.
func (w *WriteAPI) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var hashComputation = sha256.New()
	var secret []byte
	var block cipher.Block
	var mode cipher.BlockMode
	var hashdata []byte
	var orighash []byte
	var door string
	var val bool
	var ts int64
	var iv []byte
	var ok bool
	var err error

	err = req.ParseForm()
	if err != nil {
		invalidForms.Add(1)
		log.Print("Error parsing form: ", err)
		http.Error(rw, http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest)
		return
	}

	door = req.FormValue("door")
	if !doornameRe.MatchString(door) {
		invalidDoorName.Add(1)
		log.Print("Door name contains invalid characters: ", door)
		http.Error(rw, http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest)
		return
	}
	val, err = strconv.ParseBool(req.FormValue("val"))
	if err != nil {
		unparseableValue.Add(1)
		log.Print("Error parsing ", req.FormValue("val"), " as boolean: ", err)
		http.Error(rw, http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest)
		return
	}
	ts, err = strconv.ParseInt(req.FormValue("ts"), 10, 64)
	if err != nil {
		unparseableTimestamp.Add(1)
		log.Print("Error parsing ", req.FormValue("ts"), " as number: ", err)
		http.Error(rw, http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest)
		return
	}

	hashdata, err = base64.URLEncoding.DecodeString(req.FormValue("hash"))
	if err != nil {
		unparseableHash.Add(1)
		log.Print("Error parsing ", req.FormValue("hash"), " as base64: ", err)
		http.Error(rw, http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest)
		return
	}

	if len(hashdata) != sha256.Size {
		unparseableHash.Add(1)
		log.Print("Hashdata ", req.FormValue("hash"),
			" is not of expected length")
		http.Error(rw, http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest)
		return
	}

	secret, ok = w.secrets[door]
	if !ok {
		unknownDoor.Add(1)
		log.Print("Unknown door ", door)
		http.Error(rw, http.StatusText(http.StatusNotFound),
			http.StatusNotFound)
		return
	}

	block, err = aes.NewCipher(secret)
	if err != nil {
		hashVerificationFailed.Add(1)
		log.Print("Error creating AES cipher: ", err)
		http.Error(rw, http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}

	iv, err = hex.DecodeString(req.FormValue("iv"))
	if err != nil {
		unparseableHash.Add(1)
		log.Print("Error decoding IV ", req.FormValue("iv"), ": ", err)
		http.Error(rw, http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest)
		return
	}

	if len(iv) != aes.BlockSize {
		unparseableHash.Add(1)
		log.Print("IV length is not ", aes.BlockSize)
		http.Error(rw, http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest)
		return
	}

	mode = cipher.NewCBCDecrypter(block, iv)
	orighash = make([]byte, sha256.Size)
	mode.CryptBlocks(orighash, hashdata)

	_, err = io.WriteString(hashComputation, door+"\n"+req.FormValue("val")+
		"\n"+req.FormValue("ts"))
	if err != nil {
		hashVerificationFailed.Add(1)
		log.Print("Error computing request hash: ", err)
		http.Error(rw, http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}

	if !bytes.Equal(orighash, hashComputation.Sum([]byte{})) {
		hashVerificationFailed.Add(1)
		log.Print("Hash sum mismatch")
		http.Error(rw, http.StatusText(http.StatusForbidden),
			http.StatusForbidden)
		return
	}

	err = w.ts.Insert(door, time.Unix(ts, 0), val)
	if err != nil {
		insertError.Add(1)
		log.Print("Unable to insert timestamp: ", err)
		http.Error(rw, http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
	}
}
