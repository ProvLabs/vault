// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: vault/v1/interest.proto

package types

import (
	fmt "fmt"
	types "github.com/cosmos/cosmos-sdk/types"
	_ "github.com/cosmos/gogoproto/gogoproto"
	proto "github.com/cosmos/gogoproto/proto"
	github_com_cosmos_gogoproto_types "github.com/cosmos/gogoproto/types"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	io "io"
	math "math"
	math_bits "math/bits"
	time "time"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf
var _ = time.Kitchen

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type InterestPeriod struct {
	// principal is the initial amount of principal.
	Principal types.Coin `protobuf:"bytes,1,opt,name=principal,proto3" json:"principal"`
	// rate is the interest rate for the period.
	Rate string `protobuf:"bytes,2,opt,name=rate,proto3" json:"rate,omitempty"`
	// time is the payout time in seconds.
	Time int64 `protobuf:"varint,3,opt,name=time,proto3" json:"time,omitempty"`
	// start_time is the time when the interest period started.
	StartTime *time.Time `protobuf:"bytes,4,opt,name=start_time,json=startTime,proto3,stdtime" json:"start_time,omitempty"`
	// estimated_end_time is the estimated time when the interest period will end.
	EstimatedEndTime *time.Time `protobuf:"bytes,5,opt,name=estimated_end_time,json=estimatedEndTime,proto3,stdtime" json:"estimated_end_time,omitempty"`
	// estimated_interest is the amount of estimated interest that will be earned from this period.
	EstimatedInterest types.Coin `protobuf:"bytes,6,opt,name=estimated_interest,json=estimatedInterest,proto3" json:"estimated_interest"`
	// end_time is the time when the interest period will end.
	EndTime *time.Time `protobuf:"bytes,7,opt,name=end_time,json=endTime,proto3,stdtime" json:"end_time,omitempty"`
	// interest_earned is the amount of interest earned in the period.
	InterestEarned types.Coin `protobuf:"bytes,8,opt,name=interest_earned,json=interestEarned,proto3" json:"interest_earned"`
}

func (m *InterestPeriod) Reset()         { *m = InterestPeriod{} }
func (m *InterestPeriod) String() string { return proto.CompactTextString(m) }
func (*InterestPeriod) ProtoMessage()    {}
func (*InterestPeriod) Descriptor() ([]byte, []int) {
	return fileDescriptor_dbf02284312cba1f, []int{0}
}
func (m *InterestPeriod) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *InterestPeriod) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_InterestPeriod.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *InterestPeriod) XXX_Merge(src proto.Message) {
	xxx_messageInfo_InterestPeriod.Merge(m, src)
}
func (m *InterestPeriod) XXX_Size() int {
	return m.Size()
}
func (m *InterestPeriod) XXX_DiscardUnknown() {
	xxx_messageInfo_InterestPeriod.DiscardUnknown(m)
}

var xxx_messageInfo_InterestPeriod proto.InternalMessageInfo

func (m *InterestPeriod) GetPrincipal() types.Coin {
	if m != nil {
		return m.Principal
	}
	return types.Coin{}
}

func (m *InterestPeriod) GetRate() string {
	if m != nil {
		return m.Rate
	}
	return ""
}

func (m *InterestPeriod) GetTime() int64 {
	if m != nil {
		return m.Time
	}
	return 0
}

func (m *InterestPeriod) GetStartTime() *time.Time {
	if m != nil {
		return m.StartTime
	}
	return nil
}

func (m *InterestPeriod) GetEstimatedEndTime() *time.Time {
	if m != nil {
		return m.EstimatedEndTime
	}
	return nil
}

func (m *InterestPeriod) GetEstimatedInterest() types.Coin {
	if m != nil {
		return m.EstimatedInterest
	}
	return types.Coin{}
}

func (m *InterestPeriod) GetEndTime() *time.Time {
	if m != nil {
		return m.EndTime
	}
	return nil
}

func (m *InterestPeriod) GetInterestEarned() types.Coin {
	if m != nil {
		return m.InterestEarned
	}
	return types.Coin{}
}

func init() {
	proto.RegisterType((*InterestPeriod)(nil), "vault.v1.InterestPeriod")
}

func init() { proto.RegisterFile("vault/v1/interest.proto", fileDescriptor_dbf02284312cba1f) }

var fileDescriptor_dbf02284312cba1f = []byte{
	// 378 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x92, 0xb1, 0x4e, 0xeb, 0x30,
	0x18, 0x85, 0xe3, 0xdb, 0xde, 0x36, 0x35, 0x52, 0x01, 0x0b, 0x89, 0xd0, 0x21, 0xa9, 0x98, 0x3a,
	0xd9, 0x0a, 0x4c, 0x0c, 0x08, 0xa9, 0x55, 0x25, 0x58, 0x10, 0x8a, 0x98, 0x58, 0x2a, 0x27, 0x31,
	0xc1, 0x52, 0x12, 0x47, 0xb1, 0x1b, 0x89, 0xb7, 0xe8, 0x63, 0x75, 0xec, 0xc8, 0x54, 0x50, 0x3b,
	0xf2, 0x12, 0x28, 0x4e, 0x52, 0x18, 0xdb, 0xed, 0xc4, 0xbf, 0xcf, 0x39, 0x9f, 0x9c, 0x1f, 0x9e,
	0x17, 0x74, 0x1e, 0x2b, 0x52, 0xb8, 0x84, 0xa7, 0x8a, 0xe5, 0x4c, 0x2a, 0x9c, 0xe5, 0x42, 0x09,
	0x64, 0xea, 0x01, 0x2e, 0xdc, 0x81, 0x1d, 0x08, 0x99, 0x08, 0x49, 0x7c, 0x2a, 0x19, 0x29, 0x5c,
	0x9f, 0x29, 0xea, 0x92, 0x40, 0xf0, 0xb4, 0xba, 0x39, 0x38, 0x8b, 0x44, 0x24, 0xb4, 0x24, 0xa5,
	0xaa, 0x4f, 0x9d, 0x48, 0x88, 0x28, 0x66, 0x44, 0x7f, 0xf9, 0xf3, 0x57, 0xa2, 0x78, 0xc2, 0xa4,
	0xa2, 0x49, 0x56, 0x5d, 0xb8, 0xfc, 0x6e, 0xc1, 0xfe, 0x43, 0xdd, 0xf9, 0xc4, 0x72, 0x2e, 0x42,
	0x74, 0x0b, 0x7b, 0x59, 0xce, 0xd3, 0x80, 0x67, 0x34, 0xb6, 0xc0, 0x10, 0x8c, 0x8e, 0xae, 0x2e,
	0x70, 0xd5, 0x8e, 0xcb, 0x76, 0x5c, 0xb7, 0xe3, 0x89, 0xe0, 0xe9, 0xb8, 0xbd, 0x5c, 0x3b, 0x86,
	0xf7, 0xeb, 0x40, 0x08, 0xb6, 0x73, 0xaa, 0x98, 0xf5, 0x6f, 0x08, 0x46, 0x3d, 0x4f, 0xeb, 0xf2,
	0xac, 0x2c, 0xb6, 0x5a, 0x43, 0x30, 0x6a, 0x79, 0x5a, 0xa3, 0x09, 0x84, 0x52, 0xd1, 0x5c, 0xcd,
	0xf4, 0xa4, 0xad, 0x7b, 0x06, 0xb8, 0xe2, 0xc5, 0x0d, 0x2f, 0x7e, 0x6e, 0x78, 0xc7, 0xe6, 0x72,
	0xed, 0x80, 0xc5, 0xa7, 0x03, 0xbc, 0x9e, 0xf6, 0x95, 0x13, 0xe4, 0x41, 0xc4, 0xa4, 0xe2, 0x09,
	0x55, 0x2c, 0x9c, 0xb1, 0x34, 0xac, 0xc2, 0xfe, 0x1f, 0x10, 0x76, 0xb2, 0xf3, 0x4f, 0xd3, 0x50,
	0x67, 0x3e, 0xfe, 0xcd, 0x6c, 0xfe, 0x87, 0xd5, 0xd9, 0xef, 0x21, 0x4e, 0x77, 0xd6, 0xe6, 0x55,
	0xd1, 0x1d, 0x34, 0x77, 0x64, 0xdd, 0x03, 0xc8, 0xba, 0xac, 0x06, 0xba, 0x87, 0xc7, 0x0d, 0xc6,
	0x8c, 0xd1, 0x3c, 0x65, 0xa1, 0x65, 0xee, 0x47, 0xd3, 0x6f, 0x7c, 0x53, 0x6d, 0x1b, 0xdf, 0x2c,
	0x37, 0x36, 0x58, 0x6d, 0x6c, 0xf0, 0xb5, 0xb1, 0xc1, 0x62, 0x6b, 0x1b, 0xab, 0xad, 0x6d, 0x7c,
	0x6c, 0x6d, 0xe3, 0xc5, 0x89, 0xb8, 0x7a, 0x9b, 0xfb, 0x38, 0x10, 0x49, 0xb9, 0x2c, 0x45, 0x4c,
	0x7d, 0x49, 0xaa, 0xad, 0x54, 0xef, 0x19, 0x93, 0x7e, 0x47, 0xb3, 0x5e, 0xff, 0x04, 0x00, 0x00,
	0xff, 0xff, 0x66, 0x38, 0xb6, 0xa3, 0xab, 0x02, 0x00, 0x00,
}

func (m *InterestPeriod) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *InterestPeriod) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *InterestPeriod) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	{
		size, err := m.InterestEarned.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintInterest(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x42
	if m.EndTime != nil {
		n2, err2 := github_com_cosmos_gogoproto_types.StdTimeMarshalTo(*m.EndTime, dAtA[i-github_com_cosmos_gogoproto_types.SizeOfStdTime(*m.EndTime):])
		if err2 != nil {
			return 0, err2
		}
		i -= n2
		i = encodeVarintInterest(dAtA, i, uint64(n2))
		i--
		dAtA[i] = 0x3a
	}
	{
		size, err := m.EstimatedInterest.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintInterest(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x32
	if m.EstimatedEndTime != nil {
		n4, err4 := github_com_cosmos_gogoproto_types.StdTimeMarshalTo(*m.EstimatedEndTime, dAtA[i-github_com_cosmos_gogoproto_types.SizeOfStdTime(*m.EstimatedEndTime):])
		if err4 != nil {
			return 0, err4
		}
		i -= n4
		i = encodeVarintInterest(dAtA, i, uint64(n4))
		i--
		dAtA[i] = 0x2a
	}
	if m.StartTime != nil {
		n5, err5 := github_com_cosmos_gogoproto_types.StdTimeMarshalTo(*m.StartTime, dAtA[i-github_com_cosmos_gogoproto_types.SizeOfStdTime(*m.StartTime):])
		if err5 != nil {
			return 0, err5
		}
		i -= n5
		i = encodeVarintInterest(dAtA, i, uint64(n5))
		i--
		dAtA[i] = 0x22
	}
	if m.Time != 0 {
		i = encodeVarintInterest(dAtA, i, uint64(m.Time))
		i--
		dAtA[i] = 0x18
	}
	if len(m.Rate) > 0 {
		i -= len(m.Rate)
		copy(dAtA[i:], m.Rate)
		i = encodeVarintInterest(dAtA, i, uint64(len(m.Rate)))
		i--
		dAtA[i] = 0x12
	}
	{
		size, err := m.Principal.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintInterest(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func encodeVarintInterest(dAtA []byte, offset int, v uint64) int {
	offset -= sovInterest(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *InterestPeriod) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = m.Principal.Size()
	n += 1 + l + sovInterest(uint64(l))
	l = len(m.Rate)
	if l > 0 {
		n += 1 + l + sovInterest(uint64(l))
	}
	if m.Time != 0 {
		n += 1 + sovInterest(uint64(m.Time))
	}
	if m.StartTime != nil {
		l = github_com_cosmos_gogoproto_types.SizeOfStdTime(*m.StartTime)
		n += 1 + l + sovInterest(uint64(l))
	}
	if m.EstimatedEndTime != nil {
		l = github_com_cosmos_gogoproto_types.SizeOfStdTime(*m.EstimatedEndTime)
		n += 1 + l + sovInterest(uint64(l))
	}
	l = m.EstimatedInterest.Size()
	n += 1 + l + sovInterest(uint64(l))
	if m.EndTime != nil {
		l = github_com_cosmos_gogoproto_types.SizeOfStdTime(*m.EndTime)
		n += 1 + l + sovInterest(uint64(l))
	}
	l = m.InterestEarned.Size()
	n += 1 + l + sovInterest(uint64(l))
	return n
}

func sovInterest(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozInterest(x uint64) (n int) {
	return sovInterest(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *InterestPeriod) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowInterest
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: InterestPeriod: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: InterestPeriod: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Principal", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowInterest
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthInterest
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthInterest
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Principal.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Rate", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowInterest
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthInterest
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthInterest
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Rate = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Time", wireType)
			}
			m.Time = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowInterest
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Time |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field StartTime", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowInterest
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthInterest
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthInterest
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.StartTime == nil {
				m.StartTime = new(time.Time)
			}
			if err := github_com_cosmos_gogoproto_types.StdTimeUnmarshal(m.StartTime, dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field EstimatedEndTime", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowInterest
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthInterest
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthInterest
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.EstimatedEndTime == nil {
				m.EstimatedEndTime = new(time.Time)
			}
			if err := github_com_cosmos_gogoproto_types.StdTimeUnmarshal(m.EstimatedEndTime, dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field EstimatedInterest", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowInterest
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthInterest
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthInterest
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.EstimatedInterest.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 7:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field EndTime", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowInterest
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthInterest
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthInterest
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.EndTime == nil {
				m.EndTime = new(time.Time)
			}
			if err := github_com_cosmos_gogoproto_types.StdTimeUnmarshal(m.EndTime, dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 8:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field InterestEarned", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowInterest
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthInterest
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthInterest
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.InterestEarned.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipInterest(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthInterest
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipInterest(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowInterest
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowInterest
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowInterest
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthInterest
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupInterest
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthInterest
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthInterest        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowInterest          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupInterest = fmt.Errorf("proto: unexpected end of group")
)
