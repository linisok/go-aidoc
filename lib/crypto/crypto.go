package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"math/big"
	"os"

	"fmt"

	"github.com/aidoc/go-aidoc/lib/chain_common"
	"github.com/aidoc/go-aidoc/lib/crypto/sha3"
	"github.com/aidoc/go-aidoc/lib/i18"
	"github.com/aidoc/go-aidoc/lib/math"
	"github.com/aidoc/go-aidoc/lib/rlp"
)

var (
	secp256k1N, _  = new(big.Int).SetString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16)
	secp256k1halfN = new(big.Int).Div(secp256k1N, big.NewInt(2))
)
var errInvalidPubkey = errors.New("invalid secp256k1 public key")

// Keccak256计算并返回输入数据的Keccak256哈希值。
func Keccak256(data ...[]byte) []byte {
	d := sha3.NewKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

// Keccak256Hash计算并返回输入数据的Keccak256哈希，将其转换为内部哈希数据结构。
func Keccak256Hash(data ...[]byte) (h chain_common.Hash) {
	d := sha3.NewKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	d.Sum(h[:0])
	return h
}

// Keccak512计算并返回输入数据的Keccak512哈希值。
func Keccak512(data ...[]byte) []byte {
	d := sha3.NewKeccak512()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

//给定字节和随机数，创建一个 AIDOC地址
func CreateAddress(b chain_common.Address, nonce uint64) chain_common.Address {
	data, _ := rlp.EncodeToBytes([]interface{}{b, nonce})
	return chain_common.BytesToAddress(Keccak256(data)[12:])
}

// ToECDSA 使用给定的D值创建私钥。
func ToECDSA(d []byte) (*ecdsa.PrivateKey, error) {
	return toECDSA(d, true)
}

// ToECDSAUnsafe盲目地将二进制blob转换为私钥。
// 除非您确定输入有效并且希望避免因错误的原点编码（0前缀被截断）而导致出现错误，否则几乎不应该使用它。
func ToECDSAUnsafe(d []byte) *ecdsa.PrivateKey {
	priv, _ := toECDSA(d, false)
	return priv
}

// toECDSA使用给定的D值创建私钥。
// strict参数控制是否应在曲线大小处强制密钥的长度，或者它还可以接受传统编码（0前缀）。
func toECDSA(d []byte, strict bool) (*ecdsa.PrivateKey, error) {
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = S256()
	if strict && 8*len(d) != priv.Params().BitSize {
		return nil, fmt.Errorf(i18.I18_print.Sprintf("无效长度，需要 %d 位", priv.Params().BitSize))
	}
	priv.D = new(big.Int).SetBytes(d)

	// The priv.D must < N
	if priv.D.Cmp(secp256k1N) >= 0 {
		return nil, fmt.Errorf(i18.I18_print.Sprintf("私钥无效：, >=N"))
	}
	//  priv.D不得为零或否定。
	if priv.D.Sign() <= 0 {
		return nil, fmt.Errorf(i18.I18_print.Sprintf("私钥无效：, 零或负"))
	}

	priv.PublicKey.X, priv.PublicKey.Y = priv.PublicKey.Curve.ScalarBaseMult(d)
	if priv.PublicKey.X == nil {
		return nil, errors.New("私钥无效：")
	}
	return priv, nil
}

// FromECDSA将私钥导出为二进制转储。
func FromECDSA(priv *ecdsa.PrivateKey) []byte {
	if priv == nil {
		return nil
	}
	return math.PaddedBigBytes(priv.D, priv.Params().BitSize/8)
}

// UnmarshalPubkey 将字节转换为 secp256k1 公钥。
func UnmarshalPubkey(pub []byte) (*ecdsa.PublicKey, error) {
	x, y := elliptic.Unmarshal(S256(), pub)
	if x == nil {
		return nil, errInvalidPubkey
	}
	return &ecdsa.PublicKey{Curve: S256(), X: x, Y: y}, nil
}

func FromECDSAPub(pub *ecdsa.PublicKey) []byte {
	if pub == nil || pub.X == nil || pub.Y == nil {
		return nil
	}
	return elliptic.Marshal(S256(), pub.X, pub.Y)
}

// HexToECDSA 解析 secp256k1 私钥。
func HexToECDSA(hexkey string) (*ecdsa.PrivateKey, error) {
	b, err := hex.DecodeString(hexkey)
	if err != nil {
		return nil, errors.New("无效的十六进制字")
	}
	return ToECDSA(b)
}

// LoadECDSA 从给定文件加载 secp256k1 私钥。
func LoadECDSA(file string) (*ecdsa.PrivateKey, error) {
	buf := make([]byte, 64)
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	if _, err := io.ReadFull(fd, buf); err != nil {
		return nil, err
	}

	key, err := hex.DecodeString(string(buf))
	if err != nil {
		return nil, err
	}
	return ToECDSA(key)
}

// SaveECDSA使用限制权限将secp256k1私钥保存到给定文件。
// 密钥数据保存为十六进制编码。
func SaveECDSA(file string, key *ecdsa.PrivateKey) error {
	k := hex.EncodeToString(FromECDSA(key))
	return ioutil.WriteFile(file, []byte(k), 0600)
}

func GenerateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(S256(), rand.Reader)
}

// ValidateSignatureValues验证签名值是否对给定的链规则有效。 假设v值为0或1。
func ValidateSignatureValues(v byte, r, s *big.Int, homestead bool) bool {
	if r.Cmp(chain_common.Big1) < 0 || s.Cmp(chain_common.Big1) < 0 {
		return false
	}
	//拒绝s值的上限（ECDSA延展性）
	//参见secp256k1 / libsecp256k1 / include / secp256k1.h中的讨论
	if homestead && s.Cmp(secp256k1halfN) > 0 {
		return false
	}
	// Frontier：允许s在全N范围内
	return r.Cmp(secp256k1N) < 0 && s.Cmp(secp256k1N) < 0 && (v == 0 || v == 1)
}

func PubkeyToAddress(p ecdsa.PublicKey) chain_common.Address {
	pubBytes := FromECDSAPub(&p)
	return chain_common.BytesToAddress(Keccak256(pubBytes[1:])[12:])
}

func zeroBytes(bytes []byte) {
	for i := range bytes {
		bytes[i] = 0
	}
}
