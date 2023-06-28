package license

import (
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"gitlab.aswangc.cn/dataant/tools/utils"
	"gitlab.aswangc.cn/dataant/tools/utils/encryption"
)

const (
	RedisLicenseKey = "_datant_:license:"
)

var unknownLic = errors.New("unknown license")

type Subscribe struct {
	ServerName string `json:"server_name"` // 服务模块名
	Desc       string `json:"desc"`
	Limit      int    `json:"limit"` // 单个服务接口调用限制
}

type License struct {
	LegalMachine map[string]struct{}  `json:"legal_machine"` // 允许使用的机器
	Subscribes   map[string]Subscribe `json:"subscribes"`    // 订阅的服务列表
	Expire       time.Time            `json:"expire"`        // lic key 过期时间

	LicStr string `json:"-"`
}

func (l *License) String() []byte {
	buf, _ := json.Marshal(l)
	return buf
}

// 添加允许使用机器code
func (l *License) AppendMachine(machineCodes ...string) {
	if l.LegalMachine == nil {
		l.LegalMachine = make(map[string]struct{})
	}
	for _, code := range machineCodes {
		if _, ok := l.LegalMachine[code]; !ok {
			l.LegalMachine[code] = struct{}{}
		}
	}
}

// 添加订阅模块
func (l *License) AppendSubscribe(subs ...Subscribe) {
	if l.Subscribes == nil {
		l.Subscribes = make(map[string]Subscribe)
	}
	for _, sub := range subs {
		if _, ok := l.Subscribes[sub.ServerName]; !ok {
			l.Subscribes[sub.ServerName] = sub
		}
	}
}

// 生成 license 字符串
func (l *License) MakeLicense(token, priKey string) (string, error) {
	licJson := l.String()
	if len(licJson) <= 0 {
		return "", errors.New("license struct error")
	}

	key := encryption.Md5(token)
	randStr := utils.RandString(16)

	// rsa 加密
	aesEncrypt, err := encryption.AesEcrypt(licJson, []byte(key))
	if err != nil {
		return "", err
	}
	// rsa 签名
	sign, err := encryption.SignWithRSA([]byte(aesEncrypt), []byte(priKey), crypto.SHA256)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString(randStr)
	sb.WriteString(".")
	sb.WriteString(aesEncrypt)
	sb.WriteString(".")
	sb.WriteString(sign)

	l.LicStr = base64.StdEncoding.EncodeToString([]byte(sb.String()))
	return l.LicStr, nil
}

// 生成 lic 文件到当前目录
func (l *License) MakeLicenseToFile(token, priKey string) error {
	licStr, err := l.MakeLicense(token, priKey)
	if err != nil {
		return err
	}
	f, err := os.Create("license.license")
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString(licStr)
	return nil
}

// lic 暂存到 redis
func (l *License) SaveRedis(rdc *redis.ClusterClient, key string, expir ...time.Duration) error {
	var ex time.Duration = 0
	if len(expir) > 0 {
		ex = expir[0]
	}
	return rdc.Set(context.Background(), key, l.LicStr, ex).Err()
}

// 解析 license 内容
func ParseLicense(token, pubKey, licStr string) (*License, error) {
	licBuf, err := base64.StdEncoding.DecodeString(licStr)
	if err != nil {
		fmt.Println(err)
		return nil, unknownLic
	}
	lic := string(licBuf)
	licSpli := strings.Split(lic, ".")
	if len(licSpli) != 3 {
		return nil, unknownLic
	}

	key := encryption.Md5(token)
	aesEncrypt := licSpli[1]
	sign := licSpli[2]

	licDataBuf, err := encryption.AesDeCrypt(aesEncrypt, []byte(key))
	if err != nil {
		fmt.Println(err)
		return nil, unknownLic
	}
	if !encryption.VerifyWithRSA([]byte(aesEncrypt), []byte(pubKey), sign, crypto.SHA256) {
		return nil, unknownLic
	}

	license, err := utils.JsonMarshalByte[License](licDataBuf)
	if err != nil {
		fmt.Println(err)
		return nil, unknownLic
	}

	// 获取当前机器码
	machineCode, err := MachineCode()
	if err != nil {
		fmt.Println(err)
		return nil, errors.New("unauthorized machine")
	}
	// 未授权机器
	if _, ok := license.LegalMachine[machineCode]; !ok {
		return nil, errors.New("unauthorized machine")
	}
	return &license, nil
}

func MachineCode() (string, error) {
	unique, err := GetUnique()
	if err != nil {
		return "", err
	}
	return encryption.Md5(unique), nil
}
