package gcrypto

import "testing"

func TestCrypto(t *testing.T) {
	scrypto, _ := NewCrypto("ywlSRb80TaCQ4b7b")
	usr, _ := scrypto.EncryptStr("goman")
	t.Logf("%s", usr)
	if d, _ := scrypto.DecryptStr(usr); d != "goman" {
		t.Fail()
	}
	pwd, _ := scrypto.EncryptStr("123456")
	t.Logf("%s", pwd)
	if d, _ := scrypto.DecryptStr(pwd); d != "123456" {
		t.Fail()
	}
}
