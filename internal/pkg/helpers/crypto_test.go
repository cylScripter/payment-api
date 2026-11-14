package helpers

import (
	"fmt"
	"testing"
)

func TestSign(t *testing.T) {
	file, err := EncryptPEMFile("/Users/chenyinglin/project/payment-api/configs/pub_key.pem", []byte("Jy18300015530@"))
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(file)
}
