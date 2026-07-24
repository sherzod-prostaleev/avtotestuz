package click

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
)

func computeSign(req clickRequest, serviceID, secretKey string) string {
	s := req.ClickTransID + serviceID + secretKey + req.MerchantTransID
	if req.Action == "1" {
		s += req.MerchantPrepareID
	}
	s += req.Amount + req.Action + req.SignTime
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func validSign(req clickRequest, serviceID, secretKey string) bool {
	if secretKey == "" {
		return false
	}
	want := computeSign(req, serviceID, secretKey)
	return subtle.ConstantTimeCompare([]byte(want), []byte(req.SignString)) == 1
}
