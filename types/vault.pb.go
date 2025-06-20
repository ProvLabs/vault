// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: vault/v1/vault.proto

package types

import (
	fmt "fmt"
	types "github.com/cosmos/cosmos-sdk/types"
	_ "github.com/cosmos/gogoproto/gogoproto"
	proto "github.com/cosmos/gogoproto/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

// Vault represents a central holding place for assets, governed by a set of rules.
// It is based on the ERC-4626 standard and builds upon the Provenance Marker module.
type Vault struct {
	// vault_address is the bech32 address of the vault.
	VaultAddress string `protobuf:"bytes,1,opt,name=vault_address,json=vaultAddress,proto3" json:"vault_address,omitempty"`
	// marker_address is the bech32 address of the marker associated with the vault.
	// This marker holds the underlying assets.
	MarkerAddress string `protobuf:"bytes,2,opt,name=marker_address,json=markerAddress,proto3" json:"marker_address,omitempty"`
	// admin is the address that has administrative privileges over the vault.
	Admin string `protobuf:"bytes,3,opt,name=admin,proto3" json:"admin,omitempty"`
	// max_total_deposit is the absolute maximum amount of the base asset that can be deposited in the vault.
	// If empty, there is no total deposit limit.
	MaxTotalDeposit *types.Coin `protobuf:"bytes,4,opt,name=max_total_deposit,json=maxTotalDeposit,proto3" json:"max_total_deposit,omitempty"`
}

func (m *Vault) Reset()         { *m = Vault{} }
func (m *Vault) String() string { return proto.CompactTextString(m) }
func (*Vault) ProtoMessage()    {}
func (*Vault) Descriptor() ([]byte, []int) {
	return fileDescriptor_6c8870a404251180, []int{0}
}
func (m *Vault) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Vault) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Vault.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Vault) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Vault.Merge(m, src)
}
func (m *Vault) XXX_Size() int {
	return m.Size()
}
func (m *Vault) XXX_DiscardUnknown() {
	xxx_messageInfo_Vault.DiscardUnknown(m)
}

var xxx_messageInfo_Vault proto.InternalMessageInfo

func (m *Vault) GetVaultAddress() string {
	if m != nil {
		return m.VaultAddress
	}
	return ""
}

func (m *Vault) GetMarkerAddress() string {
	if m != nil {
		return m.MarkerAddress
	}
	return ""
}

func (m *Vault) GetAdmin() string {
	if m != nil {
		return m.Admin
	}
	return ""
}

func (m *Vault) GetMaxTotalDeposit() *types.Coin {
	if m != nil {
		return m.MaxTotalDeposit
	}
	return nil
}

func init() {
	proto.RegisterType((*Vault)(nil), "vault.v1.Vault")
}

func init() { proto.RegisterFile("vault/v1/vault.proto", fileDescriptor_6c8870a404251180) }

var fileDescriptor_6c8870a404251180 = []byte{
	// 278 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x3c, 0x90, 0xb1, 0x4e, 0xc3, 0x30,
	0x10, 0x86, 0x6b, 0x68, 0x11, 0x18, 0x0a, 0x22, 0xca, 0x10, 0x3a, 0xb8, 0x15, 0x08, 0xa9, 0x93,
	0xad, 0xc0, 0xc4, 0x48, 0x61, 0x63, 0xab, 0x10, 0x03, 0x4b, 0x74, 0x49, 0xac, 0x10, 0x11, 0xe7,
	0xa2, 0xd8, 0x8d, 0xca, 0x5b, 0xf0, 0x34, 0x3c, 0x43, 0xc7, 0x8e, 0x4c, 0x08, 0x25, 0x2f, 0x82,
	0x62, 0x03, 0xdb, 0x7f, 0xbf, 0xbf, 0xfb, 0xf5, 0xfb, 0xa8, 0xdf, 0xc0, 0xaa, 0x30, 0xa2, 0x09,
	0x85, 0x15, 0xbc, 0xaa, 0xd1, 0xa0, 0xb7, 0xef, 0x86, 0x26, 0x9c, 0xb0, 0x04, 0xb5, 0x42, 0x2d,
	0x62, 0xd0, 0x52, 0x34, 0x61, 0x2c, 0x0d, 0x84, 0x22, 0xc1, 0xbc, 0x74, 0xe4, 0xc4, 0xcf, 0x30,
	0x43, 0x2b, 0x45, 0xaf, 0x9c, 0x7b, 0xfe, 0x41, 0xe8, 0xe8, 0xa9, 0x8f, 0xf0, 0x2e, 0xe8, 0xd8,
	0x66, 0x45, 0x90, 0xa6, 0xb5, 0xd4, 0x3a, 0x20, 0x33, 0x32, 0x3f, 0x58, 0x1e, 0x59, 0xf3, 0xd6,
	0x79, 0xde, 0x25, 0x3d, 0x56, 0x50, 0xbf, 0xca, 0xfa, 0x9f, 0xda, 0xb1, 0xd4, 0xd8, 0xb9, 0x7f,
	0x98, 0x4f, 0x47, 0x90, 0xaa, 0xbc, 0x0c, 0x76, 0xed, 0xab, 0x1b, 0xbc, 0x07, 0x7a, 0xaa, 0x60,
	0x1d, 0x19, 0x34, 0x50, 0x44, 0xa9, 0xac, 0x50, 0xe7, 0x26, 0x18, 0xce, 0xc8, 0xfc, 0xf0, 0xea,
	0x8c, 0xbb, 0xf6, 0xbc, 0x6f, 0xcf, 0x7f, 0xdb, 0xf3, 0x3b, 0xcc, 0xcb, 0xc5, 0x70, 0xf3, 0x35,
	0x25, 0xcb, 0x13, 0x05, 0xeb, 0xc7, 0x7e, 0xf1, 0xde, 0xed, 0x2d, 0x6e, 0x36, 0x2d, 0x23, 0xdb,
	0x96, 0x91, 0xef, 0x96, 0x91, 0xf7, 0x8e, 0x0d, 0xb6, 0x1d, 0x1b, 0x7c, 0x76, 0x6c, 0xf0, 0x3c,
	0xcd, 0x72, 0xf3, 0xb2, 0x8a, 0x79, 0x82, 0x4a, 0x54, 0x35, 0x36, 0x05, 0xc4, 0xda, 0xdd, 0x4c,
	0x98, 0xb7, 0x4a, 0xea, 0x78, 0xcf, 0x7e, 0xfd, 0xfa, 0x27, 0x00, 0x00, 0xff, 0xff, 0x0e, 0x2a,
	0x56, 0x28, 0x52, 0x01, 0x00, 0x00,
}

func (m *Vault) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Vault) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Vault) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.MaxTotalDeposit != nil {
		{
			size, err := m.MaxTotalDeposit.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintVault(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x22
	}
	if len(m.Admin) > 0 {
		i -= len(m.Admin)
		copy(dAtA[i:], m.Admin)
		i = encodeVarintVault(dAtA, i, uint64(len(m.Admin)))
		i--
		dAtA[i] = 0x1a
	}
	if len(m.MarkerAddress) > 0 {
		i -= len(m.MarkerAddress)
		copy(dAtA[i:], m.MarkerAddress)
		i = encodeVarintVault(dAtA, i, uint64(len(m.MarkerAddress)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.VaultAddress) > 0 {
		i -= len(m.VaultAddress)
		copy(dAtA[i:], m.VaultAddress)
		i = encodeVarintVault(dAtA, i, uint64(len(m.VaultAddress)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintVault(dAtA []byte, offset int, v uint64) int {
	offset -= sovVault(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *Vault) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.VaultAddress)
	if l > 0 {
		n += 1 + l + sovVault(uint64(l))
	}
	l = len(m.MarkerAddress)
	if l > 0 {
		n += 1 + l + sovVault(uint64(l))
	}
	l = len(m.Admin)
	if l > 0 {
		n += 1 + l + sovVault(uint64(l))
	}
	if m.MaxTotalDeposit != nil {
		l = m.MaxTotalDeposit.Size()
		n += 1 + l + sovVault(uint64(l))
	}
	return n
}

func sovVault(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozVault(x uint64) (n int) {
	return sovVault(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *Vault) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowVault
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
			return fmt.Errorf("proto: Vault: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Vault: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field VaultAddress", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVault
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
				return ErrInvalidLengthVault
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVault
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.VaultAddress = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field MarkerAddress", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVault
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
				return ErrInvalidLengthVault
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVault
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.MarkerAddress = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Admin", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVault
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
				return ErrInvalidLengthVault
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVault
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Admin = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxTotalDeposit", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVault
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
				return ErrInvalidLengthVault
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthVault
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.MaxTotalDeposit == nil {
				m.MaxTotalDeposit = &types.Coin{}
			}
			if err := m.MaxTotalDeposit.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipVault(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthVault
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
func skipVault(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowVault
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
					return 0, ErrIntOverflowVault
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
					return 0, ErrIntOverflowVault
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
				return 0, ErrInvalidLengthVault
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupVault
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthVault
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthVault        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowVault          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupVault = fmt.Errorf("proto: unexpected end of group")
)
