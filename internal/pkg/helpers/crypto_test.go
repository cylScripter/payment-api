package helpers

import (
	"fmt"
	"testing"
)

func TestSign(t *testing.T) {
	file, err := EncryptPEMString("efba51c18a4adf83c3be39318a3584ce", []byte("Jy18300015530@"))
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(file)
}
