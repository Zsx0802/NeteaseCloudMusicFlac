package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net/url"
)

// 网易云 weapi 加密所需的固定常量.
const (
	presetKey = "0CoJUm6Qyw8W8jud" // 第一层 AES 固定密钥
	aesIV     = "0102030405060708" // AES-CBC 固定 IV
	rsaPubKey = "010001"           // RSA 公钥指数 e
	rsaModulus = "00e0b509f6259df8642dbc35662901477df22677ec152b5ff68ace615bb7b725" +
		"152b3ab17a876aea8a5aa76d2e417629ec4ee341f56135fccf695280104e0312" +
		"ecbda92557c93870114af6c9d05c4f7f0c3685b7a46bee255932575cce10b424" +
		"d813cfe4875d3e82047b97ddef52741d546b8e289dc6935b3ece0462db0a22b8e7" // RSA 模数 n
	randCharset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// pkcs7Pad 按 PKCS#7 补齐到块大小.
func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+pad)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

// aesCBCBase64 使用 AES-128-CBC 加密并输出 base64 字符串.
func aesCBCBase64(plain, key string) string {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return ""
	}
	data := pkcs7Pad([]byte(plain), block.BlockSize())
	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, []byte(aesIV)).CryptBlocks(out, data)
	return base64.StdEncoding.EncodeToString(out)
}

// rsaNoPadEncrypt 网易云使用的无填充 RSA: c = m^e mod n,
// 结果按模数 hex 长度左补零(通常 256 hex / 1024bit).
func rsaNoPadEncrypt(text, pubKey, modulus string) string {
	m := new(big.Int).SetBytes([]byte(text))
	e, _ := new(big.Int).SetString(pubKey, 16)
	n, _ := new(big.Int).SetString(modulus, 16)
	c := new(big.Int).Exp(m, e, n)
	// 网易云期望 256 位 hex(1024bit) 的密文, 左补零.
	return fmt.Sprintf("%0256x", c)
}

// randomString 生成指定长度的随机字符串(作为第二层 AES 密钥).
func randomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = randCharset[rand.Intn(len(randCharset))]
	}
	return string(b)
}

// reverseString 反转字符串.
func reverseString(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// weapiEncrypt 将任意对象序列化为 JSON 后, 按网易云 weapi 规则加密,
// 返回可直接用于 POST 表单的 params 与 encSecKey.
func weapiEncrypt(payload interface{}) url.Values {
	jsonBytes, _ := json.Marshal(payload)
	text := string(jsonBytes)

	secKey := randomString(16)
	params := aesCBCBase64(aesCBCBase64(text, presetKey), secKey)
	encSecKey := rsaNoPadEncrypt(reverseString(secKey), rsaPubKey, rsaModulus)

	v := url.Values{}
	v.Set("params", params)
	v.Set("encSecKey", encSecKey)
	return v
}
