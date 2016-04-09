// Code generated by protoc-gen-go.
// source: request_vote_responses.proto
// DO NOT EDIT!

package protobuf

import proto "github.com/gogo/protobuf/proto"
import math "math"

// discarding unused import gogoproto "github.com/gogo/protobuf/gogoproto/gogo.pb"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = math.Inf

type RequestVoteResponse struct {
	Term             *uint64 `protobuf:"varint,1,req" json:"Term,omitempty"`
	VoteGranted      *bool   `protobuf:"varint,2,req" json:"VoteGranted,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *RequestVoteResponse) Reset()         { *m = RequestVoteResponse{} }
func (m *RequestVoteResponse) String() string { return proto.CompactTextString(m) }
func (*RequestVoteResponse) ProtoMessage()    {}

func (m *RequestVoteResponse) GetTerm() uint64 {
	if m != nil && m.Term != nil {
		return *m.Term
	}
	return 0
}

func (m *RequestVoteResponse) GetVoteGranted() bool {
	if m != nil && m.VoteGranted != nil {
		return *m.VoteGranted
	}
	return false
}

func init() {
}
