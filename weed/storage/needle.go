package storage

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/chrislusf/seaweedfs/weed/glog"
	"github.com/chrislusf/seaweedfs/weed/images"
	"github.com/chrislusf/seaweedfs/weed/operation"
	"github.com/chrislusf/seaweedfs/weed/util"
)

const (
	NeedleHeaderSize      = 16 //should never change this
	NeedlePaddingSize     = 8
	NeedleChecksumSize    = 4
	MaxPossibleVolumeSize = 4 * 1024 * 1024 * 1024 * 8
)

/*
* A Needle means a uploaded and stored file.
* Needle file size is limited to 4GB for now.
 */
type Needle struct {
	Cookie uint32 `comment:"random number to mitigate brute force lookups"`
	Id     uint64 `comment:"needle id"`
	Size   uint32 `comment:"sum of DataSize,Data,NameSize,Name,MimeSize,Mime"`

	DataSize     uint32 `comment:"Data size"` //version2
	Data         []byte `comment:"The actual file data"`
	Flags        byte   `comment:"boolean flags"` //version2
	NameSize     uint8  //version2
	Name         []byte `comment:"maximum 256 characters"` //version2
	MimeSize     uint8  //version2
	Mime         []byte `comment:"maximum 256 characters"` //version2
	LastModified uint64 //only store LastModifiedBytesLength bytes, which is 5 bytes to disk
	Ttl          *TTL

	Checksum CRC    `comment:"CRC32 to check integrity"`
	Padding  []byte `comment:"Aligned to 8 bytes"`
}

func (n *Needle) String() (str string) {
	str = fmt.Sprintf("Cookie:%d, Id:%d, Size:%d, DataSize:%d, Name: %s, Mime: %s", n.Cookie, n.Id, n.Size, n.DataSize, n.Name, n.Mime)
	return
}

func ParseUpload(r *http.Request) (
	fileName string, data []byte, mimeType string, isGzipped bool,
	modifiedTime uint64, ttl *TTL, isChunkedFile bool, e error) {
	form, fe := r.MultipartReader()
	if fe != nil {
		glog.V(0).Infoln("MultipartReader [ERROR]", fe)
		e = fe
		return
	}

	//first multi-part item
	part, fe := form.NextPart()
	if fe != nil {
		glog.V(0).Infoln("Reading Multi part [ERROR]", fe)
		e = fe
		return
	}

	fileName = part.FileName()
	if fileName != "" {
		fileName = path.Base(fileName)
	}

	data, e = ioutil.ReadAll(part)
	if e != nil {
		glog.V(0).Infoln("Reading Content [ERROR]", e)
		return
	}

	//if the filename is empty string, do a search on the other multi-part items
	for fileName == "" {
		part2, fe := form.NextPart()
		if fe != nil {
			break // no more or on error, just safely break
		}

		fName := part2.FileName()

		//found the first <file type> multi-part has filename
		if fName != "" {
			data2, fe2 := ioutil.ReadAll(part2)
			if fe2 != nil {
				glog.V(0).Infoln("Reading Content [ERROR]", fe2)
				e = fe2
				return
			}

			//update
			data = data2
			fileName = path.Base(fName)
			break
		}
	}

	dotIndex := strings.LastIndex(fileName, ".")
	ext, mtype := "", ""
	if dotIndex > 0 {
		ext = strings.ToLower(fileName[dotIndex:])
		mtype = mime.TypeByExtension(ext)
	}
	contentType := part.Header.Get("Content-Type")
	if contentType != "" && mtype != contentType {
		mimeType = contentType //only return mime type if not deductable
		mtype = contentType
	}
	if part.Header.Get("Content-Encoding") == "gzip" {
		isGzipped = true
	} else if operation.IsGzippable(ext, mtype) {
		if data, e = operation.GzipData(data); e != nil {
			return
		}
		isGzipped = true
	}
	if ext == ".gz" {
		isGzipped = true
	}
	if strings.HasSuffix(fileName, ".gz") &&
		!strings.HasSuffix(fileName, ".tar.gz") {
		fileName = fileName[:len(fileName)-3]
	}
	modifiedTime, _ = strconv.ParseUint(r.FormValue("ts"), 10, 64)
	ttl, _ = ReadTTL(r.FormValue("ttl"))
	isChunkedFile, _ = strconv.ParseBool(r.FormValue("cm"))
	return
}
func NewNeedle(r *http.Request, fixJpgOrientation bool) (n *Needle, e error) {
	fname, mimeType, isGzipped, isChunkedFile := "", "", false, false
	n = new(Needle)
	fname, n.Data, mimeType, isGzipped, n.LastModified, n.Ttl, isChunkedFile, e = ParseUpload(r)
	if e != nil {
		return
	}
	if len(fname) < 256 {
		n.Name = []byte(fname)
		n.SetHasName()
	}
	if len(mimeType) < 256 {
		n.Mime = []byte(mimeType)
		n.SetHasMime()
	}
	if isGzipped {
		n.SetGzipped()
	}
	if n.LastModified == 0 {
		n.LastModified = uint64(time.Now().Unix())
	}
	n.SetHasLastModifiedDate()
	if n.Ttl != EMPTY_TTL {
		n.SetHasTtl()
	}

	if isChunkedFile {
		n.SetIsChunkManifest()
	}

	if fixJpgOrientation {
		loweredName := strings.ToLower(fname)
		if mimeType == "image/jpeg" || strings.HasSuffix(loweredName, ".jpg") || strings.HasSuffix(loweredName, ".jpeg") {
			n.Data = images.FixJpgOrientation(n.Data)
		}
	}

	n.Checksum = NewCRC(n.Data)

	commaSep := strings.LastIndex(r.URL.Path, ",")
	dotSep := strings.LastIndex(r.URL.Path, ".")
	nid := r.URL.Path[commaSep+1:]
	if dotSep > 0 {
		nid = r.URL.Path[commaSep+1 : dotSep]
	}

	e = n.ParseNid(nid)

	return
}
func (n *Needle) ParseNid(nid string) (err error) {
	length := len(nid)
	if length <= 8 {
		return errors.New("Invalid nid:" + nid)
	}
	n.Id, n.Cookie, err = ParseIdCookie(nid)
	if err != nil {
		return err
	}
	return err
}

func (n *Needle) Nid() string {
	return ToNid(n.Id, n.Cookie)
}

func ParseIdCookie(nid string) (uint64, uint32, error) {
	delta := ""
	deltaIndex := strings.LastIndex(nid, "_")
	if deltaIndex > 0 {
		nid, delta = nid[0:deltaIndex], nid[deltaIndex+1:]
	}

	if len(nid)%2 == 1 {
		nid = "0" + nid
	}
	id, cookie, ok := parseKeyHash(nid)
	if !ok {
		return 0, 0, errors.New("Invalid key and hash:" + nid)
	}

	if delta != "" {
		if d, e := strconv.ParseUint(delta, 10, 64); e == nil {
			id += d
		} else {
			return 0, 0, e
		}
	}
	return id, cookie, nil
}

func parseKeyHash(keyHash string) (uint64, uint32, bool) {
	if len(keyHash) <= 8 || len(keyHash) > 24 {
		return 0, 0, false
	}
	split := len(keyHash) - 8
	key, err := strconv.ParseUint(keyHash[:split], 16, 64)
	if err != nil {
		return 0, 0, false
	}
	hash, err := strconv.ParseUint(keyHash[split:], 16, 32)
	if err != nil {
		return 0, 0, false
	}
	return key, uint32(hash), true
}

func ToNid(key uint64, cookie uint32) string {
	bytes := make([]byte, 12)
	util.Uint64toBytes(bytes[0:8], key)
	util.Uint32toBytes(bytes[8:12], cookie)
	nonzero_index := 0
	for ; bytes[nonzero_index] == 0; nonzero_index++ {
	}
	return hex.EncodeToString(bytes[nonzero_index:])
}