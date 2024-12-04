package chain

import (
	"encoding/hex"
	"testing"
)

func TestPublicKeysFromProposalBlock(t *testing.T) {
	hex, err := hex.DecodeString("000000000000826FB5EA1379555D479E3C87A4F76E5F0C42529FDF0EB29DB76DD67A5E64A78F000000006745E0B50000000000001A8600000000000001FF000000000000588C7E625CB1463441FE927D9CA8DC638666F3F27BBA3CF1A769065340B5F06D0000000000001A870000000E0000007200000000000000000000000000000000000000000000000000000000000000000000000158734F94AF871C3D131B56131B6FB7A0291EACADD261E69DFB42A9CDF6F7FDDD0000000700002D79883D2000000000000000000000000001000000019D18C04FC87D206177303996C1D366D6CB401752000000016C39BD263CF1FA57BA28A80E1BF8472FE77854EBD98B977D8BF893EF99AB6BB10000000058734F94AF871C3D131B56131B6FB7A0291EACADD261E69DFB42A9CDF6F7FDDD0000000500005AF3107A40000000000100000000000000009DFABB9DF1E96C6391C44D7BA383FC0856F37796000000006745E2AC00000000675857AC00002D79883D20000000000158734F94AF871C3D131B56131B6FB7A0291EACADD261E69DFB42A9CDF6F7FDDD0000000700002D79883D2000000000000000000000000001000000019D18C04FC87D206177303996C1D366D6CB4017520000000B000000000000000000000001000000019D18C04FC87D206177303996C1D366D6CB4017520000000100000009000000017DCB61D3051A582599B595B913056EE2A75F4480ECEF6920DF93DB16CD9D7F9258ECF9FE5A4A46F1B998D4F77F98ECA14754CFEAFA20C34BB16A0652330629B20000000000")
	if err != nil {
		t.Fatal(err)
	}

	pks, err := PublicKeysFromPChainBlock("2JXfmg5DmADsQsSu5Kb1xRa8zJTkPBVM4FtKembYCj8KVWyHU7", hex)
	if err != nil {
		t.Fatal(err)
	}

	if len(pks) != 1 {
		t.Fatal("Expected 1 input")
	}
	if len(pks[0]) != 1 {
		t.Fatal("Expected one pk")
	}
	ethAddress, err := PublicKeyToEthAddress(pks[0][0])
	if err != nil {
		t.Fatal(err)
	}
	if ethAddress.Hex() != "0x91401C111C3adD819e73bc8C109A2c9e5BF502d9" {
		t.Fatal("Wrong address")
	}
}

func TestPublicKeysFromStandardBlock(t *testing.T) {
	hex, err := hex.DecodeString("00000000000082146647D70EBD1E0C735CE8C7CE95B951A7077AE3276A9CFDBE5920FF6654C00000000067460C080000000000001A8D000000000000048F0000000000200000000067460C08AD2801124E11CC33EDAAA752007C7D428F01BE6EA2C0B266AA63CF085D21E9460000000000001A8E000000010000000E0000007200000000000000000000000000000000000000000000000000000000000000000000000158734F94AF871C3D131B56131B6FB7A0291EACADD261E69DFB42A9CDF6F7FDDD00000007000000E987662BC000000000000000000000000100000001237605140994862F6BD6F3A3740EE6F586BBDAFF0000000575F4E911AC0D6797086FA6EA4BCF4B278955B29F5874252D4F35C61571BA3DF50000000058734F94AF871C3D131B56131B6FB7A0291EACADD261E69DFB42A9CDF6F7FDDD00000005000000003B9ACA000000000100000000844DC320BD32F382FF20E64912A9574146B4E7C207B780CE029847E55699E00D0000000058734F94AF871C3D131B56131B6FB7A0291EACADD261E69DFB42A9CDF6F7FDDD00000005000000003B8B87C00000000100000000E2589EB05CAFF60AB53EBB0B5F2AE8CAABEF7E9C50AF882800A3F7FA1AB82FF10000000058734F94AF871C3D131B56131B6FB7A0291EACADD261E69DFB42A9CDF6F7FDDD0000000500002D79882DDDC00000000100000000E33ADDBE17D9B2DF8DB8AA7E7F107792C193477945D0242188B18DC6E8BF50C90000000058734F94AF871C3D131B56131B6FB7A0291EACADD261E69DFB42A9CDF6F7FDDD00000005000000003BAA0C400000000100000000E33ADDBE17D9B2DF8DB8AA7E7F107792C193477945D0242188B18DC6E8BF50C90000000158734F94AF871C3D131B56131B6FB7A0291EACADD261E69DFB42A9CDF6F7FDDD00000005000000E8D4A51000000000010000000000000000D52B09D698E36EE1406681AA40CFA53414B582EB0000000067460CDC00000000675881DC00002D79883D20000000000158734F94AF871C3D131B56131B6FB7A0291EACADD261E69DFB42A9CDF6F7FDDD0000000700002D79883D200000000000000000000000000100000001237605140994862F6BD6F3A3740EE6F586BBDAFF0000000B00000000000000000000000100000001237605140994862F6BD6F3A3740EE6F586BBDAFF000000050000000900000001BF66061FE62D556F2B6E9DD469ED4A147557687075D3B7F32BCF9610266799EE04767F4020C111DD4CF4E0A839704FEA67C8B8AD006332285DA154AC7C19AB10000000000900000001BF66061FE62D556F2B6E9DD469ED4A147557687075D3B7F32BCF9610266799EE04767F4020C111DD4CF4E0A839704FEA67C8B8AD006332285DA154AC7C19AB10000000000900000001BF66061FE62D556F2B6E9DD469ED4A147557687075D3B7F32BCF9610266799EE04767F4020C111DD4CF4E0A839704FEA67C8B8AD006332285DA154AC7C19AB10000000000900000001BF66061FE62D556F2B6E9DD469ED4A147557687075D3B7F32BCF9610266799EE04767F4020C111DD4CF4E0A839704FEA67C8B8AD006332285DA154AC7C19AB10000000000900000001BF66061FE62D556F2B6E9DD469ED4A147557687075D3B7F32BCF9610266799EE04767F4020C111DD4CF4E0A839704FEA67C8B8AD006332285DA154AC7C19AB100000000000")
	if err != nil {
		t.Fatal(err)
	}

	pks, err := PublicKeysFromPChainBlock("pehEi5CRYEoiyofEsvmajtD7AJ1A1fNQs4dZcqKyhfcSd9PxU", hex)
	if err != nil {
		t.Fatal(err)
	}

	if len(pks) != 5 {
		t.Fatal("Expected 5 inputs")
	}
	for i, pk := range pks {
		if len(pk) != 1 {
			t.Fatalf("Expected one pk for input %d", i)
		}
		ethAddress, err := PublicKeyToEthAddress(pk[0])
		if err != nil {
			t.Fatal(err)
		}
		if ethAddress.Hex() != "0xfbD1Cd44714e241dAF3FC72f76EcAf3d186FC24C" {
			t.Fatalf("Wrong address for input %d", i)
		}
	}

}
