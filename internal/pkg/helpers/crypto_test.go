package helpers

import (
	"fmt"
	"testing"
)

func TestSign(t *testing.T) {
	file, err := EncryptPEMString("MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAnHqKLL8lf01lSxGZz0A4ICeJSduoC6XS3c8eFEdKxTprK6HELPtqO6CRPX/hPvThjwFl6Tyw/40P/QU+qbWyy2UjDzKM5taYSXRVaRDHAfnzmTYSksvrzXTnTfZKuSSZtFKgecP/OD6ZbxtgLWHSJ+6unWzyG/LuqzPSDeaD7ilBAygD2UPhtvicxY5f+BZOHjknRCTuwZOgE/3D6JmKRODPfwtbgf63uHdMHbU6C2jTvatKr4yDwrJZHm2X2XSdTfSXhn8/LEDJE54A5HGAohww3aksY0uR5+mX3NwYpPIXWtmBr4DmZ4I8Fvwzd875QhTmy+sTXPB1mrsAAo1HnQIDAQAB", []byte("Jy18300015530@"))
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println(file)
}
