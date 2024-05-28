package distribution_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Layr-Labs/eigenlayer-payment-proofs/internal/tests"
	"math/big"
	"strings"
	"testing"

	"github.com/Layr-Labs/eigenlayer-payment-proofs/pkg/distribution"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func GetTestDistribution() *distribution.Distribution {
	d := distribution.NewDistribution()

	// give some addresses many tokens
	// addr1 => token_1 => 1
	// addr1 => token_2 => 2
	// ...
	// addr1 => token_n => n
	// addr2 => token_1 => 2
	// addr2 => token_2 => 3
	// ...
	// addr2 => token_n-1 => n+1
	for i := 0; i < len(tests.TestAddresses); i++ {
		for j := 0; j < len(tests.TestTokens)-i; j++ {
			d.Set(tests.TestAddresses[i], tests.TestTokens[j], big.NewInt(int64(j+i+1)))
		}
	}

	return d
}

func GetCompleteTestDistribution() *distribution.Distribution {
	d := distribution.NewDistribution()

	for i := 0; i < len(tests.TestAddresses); i++ {
		for j := 0; j < len(tests.TestTokens); j++ {
			d.Set(tests.TestAddresses[i], tests.TestTokens[j], big.NewInt(int64(j+i+2)))
		}
	}

	return d
}

func FuzzSetAndGet(f *testing.F) {
	f.Add([]byte{69}, []byte{42, 0}, uint64(69420))

	f.Fuzz(func(t *testing.T, addressBytes, tokenBytes []byte, amounUintFuzz uint64) {
		address := common.Address{}
		address.SetBytes(addressBytes)

		token := common.Address{}
		token.SetBytes(tokenBytes)

		amount := new(big.Int).SetUint64(amounUintFuzz)

		d := distribution.NewDistribution()
		err := d.Set(address, token, amount)
		assert.NoError(t, err)

		fetched, found := d.Get(address, token)
		assert.True(t, found)
		assert.Equal(t, amount, fetched)
	})
}

func TestSetNilAmount(t *testing.T) {
	d := distribution.NewDistribution()
	err := d.Set(common.Address{}, common.Address{}, nil)
	assert.NoError(t, err)

	_, found := d.Get(common.Address{}, common.Address{})
	assert.True(t, found)
}

func TestSetAddressesInNonAlphabeticalOrder(t *testing.T) {
	d := distribution.NewDistribution()

	err := d.Set(tests.TestAddresses[1], tests.TestTokens[0], big.NewInt(1))
	assert.NoError(t, err)

	err = d.Set(tests.TestAddresses[0], tests.TestTokens[0], big.NewInt(2))
	assert.ErrorIs(t, err, distribution.ErrAddressNotInOrder)

	amount1, found := d.Get(tests.TestAddresses[1], tests.TestTokens[0])
	assert.Equal(t, big.NewInt(1), amount1)
	assert.True(t, found)

	amount2, found := d.Get(tests.TestAddresses[0], tests.TestTokens[0])
	assert.Equal(t, big.NewInt(0), amount2)
	assert.False(t, found)
}

func TestSetTokensInNonAlphabeticalOrder(t *testing.T) {
	d := distribution.NewDistribution()

	err := d.Set(tests.TestAddresses[0], tests.TestTokens[1], big.NewInt(1))
	assert.NoError(t, err)

	err = d.Set(tests.TestAddresses[0], tests.TestTokens[0], big.NewInt(2))
	assert.ErrorIs(t, err, distribution.ErrTokenNotInOrder)

	amount1, found := d.Get(tests.TestAddresses[0], tests.TestTokens[1])
	assert.Equal(t, big.NewInt(1), amount1)
	assert.True(t, found)

	amount2, found := d.Get(tests.TestAddresses[0], tests.TestTokens[0])
	assert.Equal(t, big.NewInt(0), amount2)
	assert.False(t, found)
}

func TestGetUnset(t *testing.T) {
	d := distribution.NewDistribution()

	fetched, found := d.Get(tests.TestAddresses[0], tests.TestTokens[0])
	assert.Equal(t, big.NewInt(0), fetched)
	assert.False(t, found)
}

func TestEncodeAccountLeaf(t *testing.T) {
	for i := 0; i < len(tests.TestAddresses); i++ {
		testRoot, _ := hex.DecodeString(tests.TestRootsString[i])
		leaf := distribution.EncodeAccountLeaf(tests.TestAddresses[i], testRoot)
		assert.Equal(t, distribution.EARNER_LEAF_SALT[0], leaf[0], "The first byte of the leaf should be EARNER_LEAF_SALT")
		assert.Equal(t, tests.TestAddresses[i][:], leaf[1:21])
		assert.Equal(t, testRoot, leaf[21:])
	}
}

func TestEncodeTokenLeaf(t *testing.T) {
	for i := 0; i < len(tests.TestTokens); i++ {
		testAmount, _ := new(big.Int).SetString(tests.TestAmountsString[i], 10)
		leaf := distribution.EncodeTokenLeaf(tests.TestTokens[i], testAmount)
		assert.Equal(t, distribution.TOKEN_LEAF_SALT[0], leaf[0], "The first byte of the leaf should be TOKEN_LEAF_SALT")
		assert.Equal(t, tests.TestTokens[i][:], leaf[1:21])
		assert.Equal(t, tests.TestAmountsBytes32[i], hex.EncodeToString(leaf[21:]))
	}
}

func TestGetAccountIndexBeforeMerklization(t *testing.T) {
	d := GetTestDistribution()

	accountIndex, found := d.GetAccountIndex(tests.TestAddresses[1])
	assert.False(t, found)
	assert.Equal(t, uint64(0), accountIndex)
}

func TestGetTokenIndexBeforeMerklization(t *testing.T) {
	d := GetTestDistribution()

	tokenIndex, found := d.GetTokenIndex(tests.TestAddresses[1], tests.TestTokens[1])
	assert.False(t, found)
	assert.Equal(t, uint64(0), tokenIndex)
}

func TestMerklize(t *testing.T) {
	d := GetTestDistribution()

	accountTree, tokenTrees, err := d.Merklize()
	assert.NoError(t, err)

	// check the token trees
	assert.Len(t, tokenTrees, len(tests.TestAddresses))
	for i := 0; i < len(tokenTrees); i++ {
		tokenTree, found := tokenTrees[tests.TestAddresses[i]]
		assert.True(t, found)
		assert.Len(t, tokenTree.Data, len(tests.TestTokens)-i)

		// check the data, that means the leafs are the same
		for j := 0; j < len(tests.TestTokens)-i; j++ {
			leaf := tokenTree.Data[j]
			assert.Equal(t, distribution.EncodeTokenLeaf(tests.TestTokens[j], big.NewInt(int64(j+i+1))), leaf)
		}
	}

	// check the account tree
	assert.Len(t, accountTree.Data, len(tests.TestAddresses))
	for i := 0; i < len(tests.TestAddresses); i++ {
		accountRoot := tokenTrees[tests.TestAddresses[i]].Root()
		leaf := accountTree.Data[i]
		assert.Equal(t, distribution.EncodeAccountLeaf(tests.TestAddresses[i], accountRoot), leaf)

		accountIndex, found := d.GetAccountIndex(tests.TestAddresses[i])
		assert.True(t, found)
		assert.Equal(t, uint64(i), accountIndex)

		for j := 0; j < len(tests.TestTokens)-i; j++ {
			tokenIndex, found := d.GetTokenIndex(tests.TestAddresses[i], tests.TestTokens[j])
			assert.True(t, found)
			assert.Equal(t, uint64(j), tokenIndex)
		}
	}
}

func TestNewDistributionWithData(t *testing.T) {
	distro, err := distribution.NewDistributionWithData(tests.TestJsonDistribution)
	assert.Nil(t, err)

	account, tokens, err := distro.Merklize()

	assert.Nil(t, err)
	assert.Len(t, account.Data, 1)
	addr := common.HexToAddress("0x0D6bA28b9919CfCDb6b233469Cc5Ce30b979e08E")
	assert.Len(t, tokens[addr].Data, 1)
}

func TestNewDistributionWithClaimDataLines(t *testing.T) {
	allLines := getFullTestEarnerLines()
	earnerLines := strings.Split(allLines, "\n")

	earners := make([]*distribution.EarnerLine, 0)
	for _, e := range earnerLines {
		e = e
		if e == "" {
			continue
		}
		earner := &distribution.EarnerLine{}
		err := json.Unmarshal([]byte(e), earner)
		assert.Nil(t, err)
		earners = append(earners, earner)
	}
	assert.Len(t, earners, 603)

	distro := distribution.NewDistribution()
	err := distro.LoadLines(earners)

	assert.Nil(t, err)
}

func TestDistributionLineUnMarshal(t *testing.T) {
	line := `{"earner":"0xd37f737629e0ddad7fc8adc7247d2e79c0296c35","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"2.690822691e+27"}`

	earner := &distribution.EarnerLine{}
	err := json.Unmarshal([]byte(line), earner)
	assert.Nil(t, err)
	fmt.Printf("Earner line: %+v\n", earner)
}

func getFullTestEarnerLines() string {
	return `{"earner":"0xce50089021676aa2cbac4cc72a2aa655b495bc73","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xc78b64ab536792da7b8b913f09b2954ea0b9025b","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x1920fc4fbfdce7c9a438e266d1bc53fd63c4b665","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x8fbbe44ecdae38899cc820a9cebc3575960a09a9","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xace7a881221d819892cd497291411588c4d4b746","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x392916eadfa9fa2bb95e1df9729ef7cc030e8a7e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x0c1bd1beca818bc973e6e23de280d21ae2923142","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x3ef0892a8d1b020418ddae70e28ba4fd3683a79a","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x40d5c6ecd70d70734b409af730be112b3433c5ea","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x54aad878cc1b0e884493c626d475ca43f5f2d02e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x4cf5722dbe7a22b6b0efcf42b66763a8d16202bf","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x69bf47d519797529466ed4efa1046bfa4f28a829","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x03b51d153667725fd9373e41b26a8cfff0a2aa1a","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x35066d049a196140077397a8c80bedabc4c93022","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xc821fdb6d3396323dd1cdadf2d759ffc71fa8c65","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xb8ecf20f2e3e4ce00585a99c245de880de8fc8ba","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"25831022897259709"}
{"earner":"0x67544b5349d52690b6e4eb5543b9b4b447653bc2","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x9daf77b100b2a5c3e8e9e4f89f5df1da071e686b","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xf809ec25b04d2bde3233b9183beb9f90aa2d4249","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5b4e7c735843c991a4212ab3821afed8399dd8fc","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xdeabd61ba829beb0d057038f88edcb7e64cf9242","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x70783191298a7d9a3a2da22cf22647c92a8245ba","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x3eb7b4ab5a9b5cf159daa7142f3a712bc42d01dd","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xaac26c85853920bd64297e0aa57095eaee484667","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x7894602f83c738f500a13daad931dceaa0b13066","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x0d29c82135ecb773a6f4bb1e543378fc8b89bd4d","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xe7d5efee207d6e6b2a94d87a425c8e2a9c28bc07","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xcf5b2de464f6f07f39c000fe2695833e5e26d758","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xc6ad5130cca9bf49e6b3d6517013e9e249374185","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x86b4178ae1f039758e7ea6f8a45df9a6c09cf03e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x1261711f3b72184a7e98cf583c83a3864d3a3785","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xb29e350490dee5215853a2f7a7624f685df8357b","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x44f806e0421419a8330b17d722344b9cf8c5b941","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x2a1488198552d8cbf9e21e9453df690d241333a1","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x74c28b208d58459b77fcbd38b984e9386617174c","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x8ea38052a8a0a8f45f7379e6e2f81fe52ca275b8","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x221304b44c62fc0175c693746d4414ee8c773928","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x84c2d9c7e653e38041e53c36976ddc0354fb320c","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x3dffe8b2371b7513b717bde49408a694f007cb6b","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x9fc816fe06af3b3be0e0321d0f22fc732fcc70cf","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x0be07ec5ce642666fd3a7f5ee98177673bce58da","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x20e6381f6dae8635013d5524f7bd98e49b6d27af","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x215569f3a65d68e6d03077cb4eb29e395f4535d0","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x6f5b1aded1779e43551855b646c21601c4ac3eb9","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xcb4e1c152b1a8ca30c236218bfe77866d752020f","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xe63871280cb273c75003864570cb911422f8dd51","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x3851abcf56de870abf49db51e8402cc968a8fdd4","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xaa2174e8c1687683f096b2e124e091b806a36562","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x3c42cd72639e3e8d11ab8d0072cc13bd5d8aa83c","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1712102400000,"cumulative_amount":"8462065100158564"}
{"earner":"0x93feb4ef15c70e3fdf05aacde3e546553d063a5a","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716422400000,"cumulative_amount":"5494505494505459800000000000"}
{"earner":"0xcca3c8a844abf1d141ea1b7d76148bcbb660aecb","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"4484704484704409830000000000"}
{"earner":"0xb5ab56da27ef83209ba7725f9286f58996b7066d","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"29387755102040715200000000000"}
{"earner":"0x055fc8880e53d9d063f78d4fc0b8750bda2e73c6","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"44081632653060808320000000000"}
{"earner":"0x5a0be77f71f37e342db6ab34348ca855001da51a","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"29387755102040540800000000000"}
{"earner":"0x87c83d7ae0e01cb697edab5803b7955b02524b3e","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"5381645381645356385000000000"}
{"earner":"0xba7660262c4b676fd43af18f25b5cad22cad4715","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0x464115b4f48f676048cd685b5136bcf66f63b120","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"4484704484704446370000000000"}
{"earner":"0x73a855e1259d7c55b7d85facc13f0185b11caef7","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"1195921195921190430000000000"}
{"earner":"0x35b1a2aaaeaeb2a6d1be63454bfcadaf66348fb8","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"8969408969408928490000000000"}
{"earner":"0x7db867c700a6812bf84ac4985af227c593f22a92","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"29387755102040715200000000000"}
{"earner":"0xcc5189e6b5ab0fad89e2cf6b2e7c2fe38cff28ce","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"14693877551020100240000000000"}
{"earner":"0xd37f737629e0ddad7fc8adc7247d2e79c0296c35","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"14693877551020100240000000000"}
{"earner":"0xb5ab56da27ef83209ba7725f9286f58996b7066d","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"19591836734693812960000000000"}
{"earner":"0x0d18775c9ae9fe07447995726d58e6395eed8d71","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"19591836734693812960000000000"}
{"earner":"0xcca3c8a844abf1d141ea1b7d76148bcbb660aecb","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"2690822690822645700000000000"}
{"earner":"0x8e979b94c73f7c97ad94a2fb1a3b567f97691b90","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0x627d6a16b832cae8562d15ea120ed0b25bf24079","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716422400000,"cumulative_amount":"610500610500606700000000000"}
{"earner":"0xcc5189e6b5ab0fad89e2cf6b2e7c2fe38cff28ce","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"73524085322286"}
{"earner":"0x055fc8880e53d9d063f78d4fc0b8750bda2e73c6","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"110286127983429"}
{"earner":"0xcca3c8a844abf1d141ea1b7d76148bcbb660aecb","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"73524085322286"}
{"earner":"0x0d18775c9ae9fe07447995726d58e6395eed8d71","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"8969408969408928490000000000"}
{"earner":"0xba7660262c4b676fd43af18f25b5cad22cad4715","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"13454113454113337490000000000"}
{"earner":"0x8e979b94c73f7c97ad94a2fb1a3b567f97691b90","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"17938817938817856850000000000"}
{"earner":"0x5a0be77f71f37e342db6ab34348ca855001da51a","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"44081632653060808320000000000"}
{"earner":"0xcc5189e6b5ab0fad89e2cf6b2e7c2fe38cff28ce","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"9795918367346734000000000000"}
{"earner":"0xcc5189e6b5ab0fad89e2cf6b2e7c2fe38cff28ce","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"2690822690822645700000000000"}
{"earner":"0xb5ab56da27ef83209ba7725f9286f58996b7066d","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"5381645381645356385000000000"}
{"earner":"0x0d18775c9ae9fe07447995726d58e6395eed8d71","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"5381645381645356385000000000"}
{"earner":"0x5f8127aaafa2bd771cadbd08c5df1570d9ed0770","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"36169082275000"}
{"earner":"0xd358ff208a5017f96145505d1e165f30559d2781","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"36169082275000"}
{"earner":"0x87c83d7ae0e01cb697edab5803b7955b02524b3e","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"19591836734693812960000000000"}
{"earner":"0xd358ff208a5017f96145505d1e165f30559d2781","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"19591836734693812960000000000"}
{"earner":"0xc22e6099160c9669005c7f9f89c90947f4f2796c","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"5381645381645356385000000000"}
{"earner":"0xd37f737629e0ddad7fc8adc7247d2e79c0296c35","token":"0xb6635139430eb143e1cc1d38da83ace79482ae98","snapshot":1716681600000,"cumulative_amount":"22857144"}
{"earner":"0xf58a839ba03782ea9e466d19c4c3e3c19cc3bfcb","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"4983004983004948370000000000"}
{"earner":"0x464115b4f48f676048cd685b5136bcf66f63b120","token":"0xb6635139430eb143e1cc1d38da83ace79482ae98","snapshot":1716681600000,"cumulative_amount":"22857144"}
{"earner":"0x35b1a2aaaeaeb2a6d1be63454bfcadaf66348fb8","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"36169082275000"}
{"earner":"0x2d45bcc3efa1f80065f776787bb44506dac947ed","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xc6ad5130cca9bf49e6b3d6517013e9e249374185","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xf0e4e10911dd3c113da624c5e60e1f65846e12dd","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x1e480c6ead32a5559f208d49621384eeed4844ce","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x1fad8ce8a384ad8e4819a27c074a53530695d9fe","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x5f8127aaafa2bd771cadbd08c5df1570d9ed0770","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"5929357749999"}
{"earner":"0x27977e6e4426a525d055a587d2a0537b4cb376ea","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"10672843950628560"}
{"earner":"0x8e979b94c73f7c97ad94a2fb1a3b567f97691b90","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"29646788751429"}
{"earner":"0x098adc3bac6f2d721e4fabd90ffdf0c967501379","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"5929357749999"}
{"earner":"0x9c314642ef7aebeee1695d65d943c78c68d11a18","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xc328958df2ea552d2512c5417b669a55535004a3","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x2c61fd039c89bc66eb52b117ec59860f236e28af","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x903efb57c9c628016b2b56eb6c7afb8367c4fbdd","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x8c6745bddcfb8219b06cb25f1089839796ddc0b0","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x81db2cf17e7e6e3f4aa66d450e647a69e8cb2487","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155006989872"}
{"earner":"0x132a1c379e152661549723f25691c149745e7b6e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"5929357750348566"}
{"earner":"0x35066d049a196140077397a8c80bedabc4c93022","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x038d8659d3be43db4c77c38f1a73514901194321","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x191439c1c5b99946e89e5e737ac6f5e8809f4cf0","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb05a4a320632e4d4c1f35f91a4874ee2bd0189b8","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x4cf5722dbe7a22b6b0efcf42b66763a8d16202bf","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x841938a6a1c90a5537c779e9158791353bef2278","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x42c116a998cfbe5b990b5bb0a4d964fa47f2f84f","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x475e444693e4729a5fb83b66000ff50800a22164","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x9b8a120561bae09d29d606c0f35beca4585db865","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xad9770d6b5514724c7b766f087bea8a784038cbe","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"592935775034286"}
{"earner":"0xc14381e1a715a634cb2fac6865dae6b6fb943b5d","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x6f5b1aded1779e43551855b646c21601c4ac3eb9","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xbf1a34caabf0b22443ad8443c7ceb1756109db32","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x856af05d3d58bdafc91665b98aa6062abc4a97f0","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xc67f4eeae5785497709af9839ad5bbef32c3b156","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x895945fe5b255230721cf9ac505e1839d02489fc","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xab598d2cf11102b8df8782b1a7e7f67849cf828a","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xf7eaf102308678d21fa1bebaa6af31477572b893","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x221304b44c62fc0175c693746d4414ee8c773928","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x70783191298a7d9a3a2da22cf22647c92a8245ba","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb8ecf20f2e3e4ce00585a99c245de880de8fc8ba","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"592935775034286"}
{"earner":"0xcf5b2de464f6f07f39c000fe2695833e5e26d758","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xa0f51ae71344ba25ccb930ee3eb30413639ce124","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xdd012dc45a980c1aa02f65340d0e41e29fde279e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xe8ded90c323af3f405c5b5461e3a3f50ce41d6a2","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x1aa3e2fffe81531ca1e40c6aad6c39a10c54969b","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x6c469110df64f805bf7f3b718fd5f6c6b95f4abc","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x84c2d9c7e653e38041e53c36976ddc0354fb320c","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xc821fdb6d3396323dd1cdadf2d759ffc71fa8c65","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x0c1bd1beca818bc973e6e23de280d21ae2923142","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x5454c41c98a22fb6d9ac3f6030acd364c1e03e0d","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x2f0a12f4e069694260b61d38e7bf64d16cc9ad60","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xd9ea89bb4a9b613fbced031fbb083b25e7878e7b","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x89cb34b3306e839e4d735374ac20a47911c7ea2f","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x032fb3b9cebf96918b7b3f101555a0f01eb0e6b9","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb2df8e64c8a4310edfa744158472113b6bd72fc2","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"17788073250000"}
{"earner":"0x47ce00402d72879ab3b7ff37758649b6a84682a2","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x6b11d29a8b972535a37addf596bec76f892822f7","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x20e6381f6dae8635013d5524f7bd98e49b6d27af","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x2891763cb440e07bd08bf5833118d3757635dd5f","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x67179ea9d048a7ba08ad7b15f9b0e015e1670795","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xcb143c69ab39fdfb4a34ca62093b8cf21609bec9","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb889189803685c04a654b8c69ea494c7265598bf","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xab598d2cf11102b8df8782b1a7e7f67849cf828a","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xafa192c5bc876e409f7aa056bfff9d06df5789bb","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xb80e9e9c67d2d7bbb75e196a56c1d1c892a6c9e1","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x7ab00eafc213ac2a0d61b08af68ea9951f6dac85","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xc67f4eeae5785497709af9839ad5bbef32c3b156","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xd2b5653fad40e1b0f9ffb5f942288813629514d0","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x81db2cf17e7e6e3f4aa66d450e647a69e8cb2487","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"1841903976777370150"}
{"earner":"0x2ca01722195808c0550c26f2edaaaaf5fa2ab414","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x2891763cb440e07bd08bf5833118d3757635dd5f","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x1e480c6ead32a5559f208d49621384eeed4844ce","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x903efb57c9c628016b2b56eb6c7afb8367c4fbdd","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xf8c5b29a87b86800d132a30bff61452cc858e82e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x38f6d15fcc481097d3c7274fc926363845b3f6af","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xa816e7760711526348ed126277f5b28cd3de8769","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x200a1d774936e0874fdcac824f0daea5aece8a4b","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xe5d590c5a14818aef02d97d31bd14ac4a46721a9","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x032a40d1de9735b7ac7992674a9baddf3877b162","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x475e444693e4729a5fb83b66000ff50800a22164","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x42c116a998cfbe5b990b5bb0a4d964fa47f2f84f","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xea030a0dff5c5bc6043249812e391a0e6db66a0a","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x040eb26a2a9afe10ef16b04a18be0b1f22391728","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xfb525151815256b8992e5fe8f5b1ac3b8ecd66fa","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xed407fb088a75df3746d1b5b5312d7e20e23f19d","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x9fb00258778754d28d9701d1218c746a017c04b2","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x27977e6e4426a525d055a587d2a0537b4cb376ea","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"765748110280417137"}
{"earner":"0xcb14cfaac122e52024232583e7354589aede74ff","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"189556913597988248"}
{"earner":"0xc79eaf0f7bc177a528b053cefb2cf129b1d6bff2","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xd15415c83a874d2336610b55b85724dc2c98a2d4","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xd1676b55e9c71b6964c91f5e916ca28e6b5d3369","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x8cfd23229339f928b7e7cfc446098b8d7ac9ee40","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xbf89d4b567e0aa461306f6637874cafb5b6ad14c","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5e5dcfebd13a0f432ce66414cb1687f9164b7ee8","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x4f9c94b164143d7d2addba0791da1ad803586e68","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x9d690a34e645abf5b5074464d62afb3d05da975b","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x08300f3797dcee9cc813ebaae25127b4e3b550e3","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"157033895467124007"}
{"earner":"0x9a27c414894d2292d1eefe0c1f3e60d3e59c4455","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x2d1ea5f682ac8ef21308551e4d641203dedc6c07","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x038d8659d3be43db4c77c38f1a73514901194321","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x39559962904825093014d50230284cebac340eaa","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x8c6745bddcfb8219b06cb25f1089839796ddc0b0","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xf0e4e10911dd3c113da624c5e60e1f65846e12dd","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x841938a6a1c90a5537c779e9158791353bef2278","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xbb058027d72ee62475f10400778ab5b13054242f","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x2967331481a4658f0e3c1dbbc7eb00b48fe863ba","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x122ffca660a784352337f56d431ffb57f3916a8e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x2d45bcc3efa1f80065f776787bb44506dac947ed","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x032fb3b9cebf96918b7b3f101555a0f01eb0e6b9","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5454c41c98a22fb6d9ac3f6030acd364c1e03e0d","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x1c5ccffbf9051d36d23245c735d3a775beab150f","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x1fad8ce8a384ad8e4819a27c074a53530695d9fe","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xefb80bdd3f39c2b0e3659029c7f9b8f7ab19f4e7","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xb2ed8234019a51689f8efc17252f585a4aabe037","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xfce6656b2ea82d2be38f2887ebe0a586a388e206","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"2914808138707696394"}
{"earner":"0xd6db4dfc3c12dededd444b690fa78e46ba4ef063","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xa0f51ae71344ba25ccb930ee3eb30413639ce124","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x9f7a49a58d5647f2ba9ffe35a50fc32a4e08200e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5f8127aaafa2bd771cadbd08c5df1570d9ed0770","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"5381645381645356385000000000"}
{"earner":"0x93feb4ef15c70e3fdf05aacde3e546553d063a5a","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716422400000,"cumulative_amount":"7142857142857064900000000000"}
{"earner":"0x098adc3bac6f2d721e4fabd90ffdf0c967501379","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0x1e38e4a44ecfbd94154c1f347fbeeb74c0fcc713","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"996600996600992110000000000"}
{"earner":"0x5f8127aaafa2bd771cadbd08c5df1570d9ed0770","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"8969408969408928490000000000"}
{"earner":"0x0d18775c9ae9fe07447995726d58e6395eed8d71","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"29387755102040715200000000000"}
{"earner":"0x7db867c700a6812bf84ac4985af227c593f22a92","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"19591836734693812960000000000"}
{"earner":"0xba7660262c4b676fd43af18f25b5cad22cad4715","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"8072468072468001420000000000"}
{"earner":"0xcca3c8a844abf1d141ea1b7d76148bcbb660aecb","token":"0xb6635139430eb143e1cc1d38da83ace79482ae98","snapshot":1716681600000,"cumulative_amount":"22857144"}
{"earner":"0xb004f902b643c74f5c95c2376ac748044dc310c4","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"2989802989802976330000000000"}
{"earner":"0x464115b4f48f676048cd685b5136bcf66f63b120","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"2690822690822667510000000000"}
{"earner":"0x73a855e1259d7c55b7d85facc13f0185b11caef7","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"2607709750566869220000000000"}
{"earner":"0x8e979b94c73f7c97ad94a2fb1a3b567f97691b90","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"183810213314427"}
{"earner":"0x87c83d7ae0e01cb697edab5803b7955b02524b3e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"36169082275000"}
{"earner":"0xc22e6099160c9669005c7f9f89c90947f4f2796c","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"36169082275000"}
{"earner":"0xba7660262c4b676fd43af18f25b5cad22cad4715","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"110286127983429"}
{"earner":"0x0b1302c23d9eb4b42a74cbefc4f9b3081ff1bf18","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"7945425532141974828"}
{"earner":"0x93feb4ef15c70e3fdf05aacde3e546553d063a5a","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"36169082275000"}
{"earner":"0xcc5189e6b5ab0fad89e2cf6b2e7c2fe38cff28ce","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"4484704484704409830000000000"}
{"earner":"0x5f8127aaafa2bd771cadbd08c5df1570d9ed0770","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"29387755102040715200000000000"}
{"earner":"0x8e979b94c73f7c97ad94a2fb1a3b567f97691b90","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"39183673469387622640000000000"}
{"earner":"0x5f8127aaafa2bd771cadbd08c5df1570d9ed0770","token":"0xb6635139430eb143e1cc1d38da83ace79482ae98","snapshot":1716681600000,"cumulative_amount":"45714280"}
{"earner":"0xd358ff208a5017f96145505d1e165f30559d2781","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0xb004f902b643c74f5c95c2376ac748044dc310c4","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"1793881793881785645000000000"}
{"earner":"0x098adc3bac6f2d721e4fabd90ffdf0c967501379","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"8969408969408928490000000000"}
{"earner":"0x8e979b94c73f7c97ad94a2fb1a3b567f97691b90","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"58775510204081430400000000000"}
{"earner":"0x7db867c700a6812bf84ac4985af227c593f22a92","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"5381645381645356385000000000"}
{"earner":"0xcc5189e6b5ab0fad89e2cf6b2e7c2fe38cff28ce","token":"0xb6635139430eb143e1cc1d38da83ace79482ae98","snapshot":1716681600000,"cumulative_amount":"22857144"}
{"earner":"0x08d679cf1f208684ff8334849c2ef0568cdaabe7","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"9795918367346848240000000000"}
{"earner":"0xf58a839ba03782ea9e466d19c4c3e3c19cc3bfcb","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"16326530612244784960000000000"}
{"earner":"0x464115b4f48f676048cd685b5136bcf66f63b120","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"9795918367346849280000000000"}
{"earner":"0x627d6a16b832cae8562d15ea120ed0b25bf24079","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716422400000,"cumulative_amount":"793650793650785100000000000"}
{"earner":"0xf58a839ba03782ea9e466d19c4c3e3c19cc3bfcb","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"3911564625850303830000000000"}
{"earner":"0x0d29c82135ecb773a6f4bb1e543378fc8b89bd4d","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb2ed8234019a51689f8efc17252f585a4aabe037","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x45cfd19595542968b57a51e84c0e5f3c59ddb224","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x3b0a021b26d04470aea4852c715b5c2ee41ded47","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x4ea9a7f41dc0b74d3016bd35157043593efa0514","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"889403662551429"}
{"earner":"0x4fc7a152c4307323df9779e8ced52ba09c5c89e0","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xce50089021676aa2cbac4cc72a2aa655b495bc73","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xfce6656b2ea82d2be38f2887ebe0a586a388e206","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"48620733552865668"}
{"earner":"0xfb6e0d1746133e1ad5c749efc3dd789c84e88090","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x032a40d1de9735b7ac7992674a9baddf3877b162","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb5ab56da27ef83209ba7725f9286f58996b7066d","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"5929357749999"}
{"earner":"0xd65fd6728d592ce7e7e94de22871adbee7fd77d2","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x769872c2df9c9b328aa0611b01ae97d48bf7f16b","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xcc5189e6b5ab0fad89e2cf6b2e7c2fe38cff28ce","token":"0x92df4da39fe6f6c039c0c6d5da3cd481d641aaf8","snapshot":1716681600000,"cumulative_amount":"428571428571423900000000000000000000"}
{"earner":"0x39559962904825093014d50230284cebac340eaa","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xaa2174e8c1687683f096b2e124e091b806a36562","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xf9d0b0528dc20cc02c58fc9b4cca61e62c90368b","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x5daac14781a5c4af2b0673467364cba46da935db","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"2365220806614285"}
{"earner":"0x5837d0e331a7c427ea33ea0e651f7ae4869c469a","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xfedd6612faff04d077a684eea615944bf3236919","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"592935775034286"}
{"earner":"0xe896a22eecb31152812f5fc978a83c3b6cb1fc2d","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x0be07ec5ce642666fd3a7f5ee98177673bce58da","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x02c0e523aa4797727c464816ad37f40723b472ef","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x355c21675662ba6d9c67b5216e7bd740801eba7e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x8e730ec4aab4b8d6a6b2ba9bfed56e311c4884e2","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xf4a51b5e127749db48c1a1a950a5fc692c482a52","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x69bf47d519797529466ed4efa1046bfa4f28a829","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x4173a582b6110db8bf290a7371a0a38cd714fda6","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"592935775034286"}
{"earner":"0x5b4e7c735843c991a4212ab3821afed8399dd8fc","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x0597b599d0976fc2493be0950088cc9c911c55d1","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x7894602f83c738f500a13daad931dceaa0b13066","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xcb4e1c152b1a8ca30c236218bfe77866d752020f","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x6bc5422f8e0aa6c69731d2c72d89b21f47f287db","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xd37f737629e0ddad7fc8adc7247d2e79c0296c35","token":"0x92df4da39fe6f6c039c0c6d5da3cd481d641aaf8","snapshot":1716681600000,"cumulative_amount":"428571428571423900000000000000000000"}
{"earner":"0x9fb00258778754d28d9701d1218c746a017c04b2","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x333fe9bab8e1b1894ba7e0a523ddd8cb6e382912","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x6890c8a073cf24f451702d080a48d84de9698e2c","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x3ab04e0808b10503717a938eda776842ae01e591","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x5f319de97a21b84ae24821f4f58ef83ef2efc86e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x8fbbe44ecdae38899cc820a9cebc3575960a09a9","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x08300f3797dcee9cc813ebaae25127b4e3b550e3","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"2371743100138569"}
{"earner":"0x2a1488198552d8cbf9e21e9453df690d241333a1","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x9a27c414894d2292d1eefe0c1f3e60d3e59c4455","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xbbcfb6544d350eb992f0fae588b479087da6a130","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x807fc1d336f2d6a48412af0c1a55663532cec929","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x7db867c700a6812bf84ac4985af227c593f22a92","token":"0x92df4da39fe6f6c039c0c6d5da3cd481d641aaf8","snapshot":1716681600000,"cumulative_amount":"857142857142854090000000000000000000"}
{"earner":"0xa98a33de6e6e2c6b03361e9fbba207dc3251db33","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x52033af3b4c2b7583f188bab806fa972e1f2f27a","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x74c28b208d58459b77fcbd38b984e9386617174c","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xa5961eba49ce5d6a56ccab2ff5fbdf27f0091f0c","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x1c5ccffbf9051d36d23245c735d3a775beab150f","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x7821acb8ed921e3e634ef14bb796603869bf67b5","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x155b4a1e465907861e24e6c25264be3df30071a4","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x215569f3a65d68e6d03077cb4eb29e395f4535d0","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x2349a5b27700b3a19178c07a81bd407a916ad07a","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xd2b5653fad40e1b0f9ffb5f942288813629514d0","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x4f4f39ad3b8625218f6dfae4e775e0caea02b562","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xe9f28aad3b4c176d562260ab0dabe1e6f2c9f9cd","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x5f8127aaafa2bd771cadbd08c5df1570d9ed0770","token":"0x92df4da39fe6f6c039c0c6d5da3cd481d641aaf8","snapshot":1716681600000,"cumulative_amount":"857142857142854090000000000000000000"}
{"earner":"0xba7660262c4b676fd43af18f25b5cad22cad4715","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"17788073250000"}
{"earner":"0x86b4178ae1f039758e7ea6f8a45df9a6c09cf03e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x4d56e201872dfc2b8110bc1048e66bf34b2b054c","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x87c83d7ae0e01cb697edab5803b7955b02524b3e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"5929357749999"}
{"earner":"0xd244a5c331c5c6a7d5907e985129a4b9afc6478c","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xd358ff208a5017f96145505d1e165f30559d2781","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"5929357749999"}
{"earner":"0x29523eb853e5bba1ad3c52f5752288946771feae","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x4fc7a152c4307323df9779e8ced52ba09c5c89e0","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x210f86eef811b63da42195d52e67d1896a909e48","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x96856db7ffdfe3aca850a5101bbc8c7b365d2619","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xd9ea89bb4a9b613fbced031fbb083b25e7878e7b","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x972413dc71d883157ab47ebf34aef12d687c1980","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xdd012dc45a980c1aa02f65340d0e41e29fde279e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x6c469110df64f805bf7f3b718fd5f6c6b95f4abc","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x1092a6b0c01b808b5277a0544b6eb2f2e4570225","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xcfe9c63830cb9a9944f3aa43c70265ec9bf81e6e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x4f4f39ad3b8625218f6dfae4e775e0caea02b562","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xa98a33de6e6e2c6b03361e9fbba207dc3251db33","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xcb28c4a86406ba274507d4933e64eedfca16f696","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x769872c2df9c9b328aa0611b01ae97d48bf7f16b","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x333fe9bab8e1b1894ba7e0a523ddd8cb6e382912","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x3b0a021b26d04470aea4852c715b5c2ee41ded47","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x7b5795fece8af52e7c70c6cebc03c0a214c72065","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x44f4f108935dd249c69725d0b196ef4a0dc14115","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xf9d0b0528dc20cc02c58fc9b4cca61e62c90368b","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x17c0ba040501f7c289e712bcf004d76184524eb3","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x2349a5b27700b3a19178c07a81bd407a916ad07a","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5f319de97a21b84ae24821f4f58ef83ef2efc86e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xb38fb683657d9d0781dbd1a78119d4a1c70e4331","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x6e8bc2dba7878d3676fbaf308f79ae752d95ab7d","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x4173a582b6110db8bf290a7371a0a38cd714fda6","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"25831022897259709"}
{"earner":"0xd244a5c331c5c6a7d5907e985129a4b9afc6478c","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xfc66ac6209a47a1f4fc26844fde69639925f1615","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xd65fd6728d592ce7e7e94de22871adbee7fd77d2","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xcb143c69ab39fdfb4a34ca62093b8cf21609bec9","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x9b8a120561bae09d29d606c0f35beca4585db865","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x2222aac0c980cc029624b7ff55b88bc6f63c538f","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"340614876061182948"}
{"earner":"0xbbcfb6544d350eb992f0fae588b479087da6a130","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x4d56e201872dfc2b8110bc1048e66bf34b2b054c","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x190e38505e13bcd85997d1282298286b037b26bf","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xf4a51b5e127749db48c1a1a950a5fc692c482a52","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x53dd46f4c610d10b1e35ab8d0fd1de0e8a648ded","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xb43e18f5d634524c8fb2c039ff25eaab8b7a6010","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xad76f194b8ab979d2fbb900756af849ee875f8a0","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x0fe3f4dff1d3c7ad5497e5f76df6e33bc823e8c7","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x2d1055d94bb303a3633a692c6f1fdcf9055252d6","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xb42fa21d25d86995d64e3eb0634008b4799f2ec0","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xf183173ae3e749afcaf4652033836b2f880c08d8","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x132a1c379e152661549723f25691c149745e7b6e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"541546314748579064"}
{"earner":"0xfedd6612faff04d077a684eea615944bf3236919","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"25831022897259709"}
{"earner":"0x5837d0e331a7c427ea33ea0e651f7ae4869c469a","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x072fbd88688762168704431dc63eeb8335c6ed00","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x15b82df5d3d8e5414c5f2627a4eb6ef86300b6ad","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5e4e436b7444d834733e57eacac09436215e6a2e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5ae37715142262261052badd8c16f77db5b17ac2","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5db688ba62115879650d799d1a128943ebc5ce59","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xf1f623fe7310f83af3c913241510c7e4cd179bf9","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x895945fe5b255230721cf9ac505e1839d02489fc","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x8e730ec4aab4b8d6a6b2ba9bfed56e311c4884e2","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5afa1a92750dce5a6c94ad4c2b18bc914cf0f20a","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x02c0e523aa4797727c464816ad37f40723b472ef","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xc813c5305874f98d256deed64b16645e9435ff3b","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x287e19af31b0fee8f2836c9c6759ef28718e169b","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x89cb34b3306e839e4d735374ac20a47911c7ea2f","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x47ce00402d72879ab3b7ff37758649b6a84682a2","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5a0be77f71f37e342db6ab34348ca855001da51a","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"13454113454113337490000000000"}
{"earner":"0xd358ff208a5017f96145505d1e165f30559d2781","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"29387755102040715200000000000"}
{"earner":"0x098adc3bac6f2d721e4fabd90ffdf0c967501379","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"5381645381645356385000000000"}
{"earner":"0x055fc8880e53d9d063f78d4fc0b8750bda2e73c6","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0x5a0be77f71f37e342db6ab34348ca855001da51a","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0xb004f902b643c74f5c95c2376ac748044dc310c4","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"3911564625850303830000000000"}
{"earner":"0xb5ab56da27ef83209ba7725f9286f58996b7066d","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"8969408969408928490000000000"}
{"earner":"0x93feb4ef15c70e3fdf05aacde3e546553d063a5a","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716422400000,"cumulative_amount":"3296703296703275525000000000"}
{"earner":"0x35b1a2aaaeaeb2a6d1be63454bfcadaf66348fb8","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"5381645381645356385000000000"}
{"earner":"0xba7660262c4b676fd43af18f25b5cad22cad4715","token":"0xb6635139430eb143e1cc1d38da83ace79482ae98","snapshot":1716681600000,"cumulative_amount":"68571432"}
{"earner":"0x0d18775c9ae9fe07447995726d58e6395eed8d71","token":"0xb6635139430eb143e1cc1d38da83ace79482ae98","snapshot":1716681600000,"cumulative_amount":"45714280"}
{"earner":"0xb2df8e64c8a4310edfa744158472113b6bd72fc2","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0x1e38e4a44ecfbd94154c1f347fbeeb74c0fcc713","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"1303854875283434610000000000"}
{"earner":"0x5a0be77f71f37e342db6ab34348ca855001da51a","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"110286127983429"}
{"earner":"0x7db867c700a6812bf84ac4985af227c593f22a92","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"36169082275000"}
{"earner":"0x098adc3bac6f2d721e4fabd90ffdf0c967501379","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"36169082275000"}
{"earner":"0x055fc8880e53d9d063f78d4fc0b8750bda2e73c6","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"13454113454113337490000000000"}
{"earner":"0xb2df8e64c8a4310edfa744158472113b6bd72fc2","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"13454113454113337490000000000"}
{"earner":"0x098adc3bac6f2d721e4fabd90ffdf0c967501379","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"19591836734693812960000000000"}
{"earner":"0x5f8127aaafa2bd771cadbd08c5df1570d9ed0770","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"19591836734693812960000000000"}
{"earner":"0x5a0be77f71f37e342db6ab34348ca855001da51a","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"8072468072468001420000000000"}
{"earner":"0x8e979b94c73f7c97ad94a2fb1a3b567f97691b90","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"10763290763290713070000000000"}
{"earner":"0x7db867c700a6812bf84ac4985af227c593f22a92","token":"0xb6635139430eb143e1cc1d38da83ace79482ae98","snapshot":1716681600000,"cumulative_amount":"45714280"}
{"earner":"0x73a855e1259d7c55b7d85facc13f0185b11caef7","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"6530612244897937600000000000"}
{"earner":"0x627d6a16b832cae8562d15ea120ed0b25bf24079","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716422400000,"cumulative_amount":"366300366300363975000000000"}
{"earner":"0xd37f737629e0ddad7fc8adc7247d2e79c0296c35","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"9795918367346734000000000000"}
{"earner":"0x87c83d7ae0e01cb697edab5803b7955b02524b3e","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0x0d18775c9ae9fe07447995726d58e6395eed8d71","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0x464115b4f48f676048cd685b5136bcf66f63b120","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"14693877551020272800000000000"}
{"earner":"0x73a855e1259d7c55b7d85facc13f0185b11caef7","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"4353741496598625280000000000"}
{"earner":"0xf183173ae3e749afcaf4652033836b2f880c08d8","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xc813c5305874f98d256deed64b16645e9435ff3b","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xed407fb088a75df3746d1b5b5312d7e20e23f19d","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x122ffca660a784352337f56d431ffb57f3916a8e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x2222aac0c980cc029624b7ff55b88bc6f63c538f","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"18973944801117600"}
{"earner":"0x479d40d0664b893f028b476f65ab422be7a36dca","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x5a0be77f71f37e342db6ab34348ca855001da51a","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"17788073250000"}
{"earner":"0xe5d590c5a14818aef02d97d31bd14ac4a46721a9","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x8cfd23229339f928b7e7cfc446098b8d7ac9ee40","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb29e350490dee5215853a2f7a7624f685df8357b","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb564c8a61682c4977b0fcbf4118eb1ff8ec8d803","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x40d5c6ecd70d70734b409af730be112b3433c5ea","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x96856db7ffdfe3aca850a5101bbc8c7b365d2619","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xd37f737629e0ddad7fc8adc7247d2e79c0296c35","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"11858715500001"}
{"earner":"0x5e5dcfebd13a0f432ce66414cb1687f9164b7ee8","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x9d690a34e645abf5b5074464d62afb3d05da975b","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x972413dc71d883157ab47ebf34aef12d687c1980","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xace7a881221d819892cd497291411588c4d4b746","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xe7d5efee207d6e6b2a94d87a425c8e2a9c28bc07","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x54aad878cc1b0e884493c626d475ca43f5f2d02e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xd6db4dfc3c12dededd444b690fa78e46ba4ef063","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xc22e6099160c9669005c7f9f89c90947f4f2796c","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"5929357749999"}
{"earner":"0xc79eaf0f7bc177a528b053cefb2cf129b1d6bff2","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xcca3c8a844abf1d141ea1b7d76148bcbb660aecb","token":"0x92df4da39fe6f6c039c0c6d5da3cd481d641aaf8","snapshot":1716681600000,"cumulative_amount":"428571428571423900000000000000000000"}
{"earner":"0x2d1055d94bb303a3633a692c6f1fdcf9055252d6","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb97bd2cc995ffdcad74e16fe3a721b20b09efd25","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xf8c5b29a87b86800d132a30bff61452cc858e82e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xf7dc9bf58f3200ab64d9b007baf287328c999db3","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xf1f623fe7310f83af3c913241510c7e4cd179bf9","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x1261711f3b72184a7e98cf583c83a3864d3a3785","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x24f84af59f680e9fbf5c66ec26811016a7532c5c","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb43e18f5d634524c8fb2c039ff25eaab8b7a6010","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xbf89d4b567e0aa461306f6637874cafb5b6ad14c","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xbb058027d72ee62475f10400778ab5b13054242f","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x2967331481a4658f0e3c1dbbc7eb00b48fe863ba","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x67544b5349d52690b6e4eb5543b9b4b447653bc2","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x44f806e0421419a8330b17d722344b9cf8c5b941","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xafa192c5bc876e409f7aa056bfff9d06df5789bb","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x3eb7b4ab5a9b5cf159daa7142f3a712bc42d01dd","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x29523eb853e5bba1ad3c52f5752288946771feae","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xa816e7760711526348ed126277f5b28cd3de8769","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x9f7a49a58d5647f2ba9ffe35a50fc32a4e08200e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xcb14cfaac122e52024232583e7354589aede74ff","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"3201853185188568"}
{"earner":"0x10854d7c1580c12963c1bb4446128c6a341df3b1","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x200a1d774936e0874fdcac824f0daea5aece8a4b","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xf2b08015a8b9b10fd534efa4b8f447172be6afcc","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x7db867c700a6812bf84ac4985af227c593f22a92","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"5929357749999"}
{"earner":"0x82e0dabea2c874d391d677fa3887df5a3f6283c2","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x5c2aa9b2b1eaa5d43ab95401df3c509fc1c43ef7","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x2ca01722195808c0550c26f2edaaaaf5fa2ab414","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x5db688ba62115879650d799d1a128943ebc5ce59","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x3ef0892a8d1b020418ddae70e28ba4fd3683a79a","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x44f4f108935dd249c69725d0b196ef4a0dc14115","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x040eb26a2a9afe10ef16b04a18be0b1f22391728","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x287e19af31b0fee8f2836c9c6759ef28718e169b","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x9fc816fe06af3b3be0e0321d0f22fc732fcc70cf","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xad76f194b8ab979d2fbb900756af849ee875f8a0","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xac187c4278e9d015bd83dab3db5dc99434f7e765","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xe63871280cb273c75003864570cb911422f8dd51","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xe9f28aad3b4c176d562260ab0dabe1e6f2c9f9cd","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x82e0dabea2c874d391d677fa3887df5a3f6283c2","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x9c314642ef7aebeee1695d65d943c78c68d11a18","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x6890c8a073cf24f451702d080a48d84de9698e2c","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x2a88d4b71a62afbe0c943f2975e3d0592d8e71a7","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xf2b08015a8b9b10fd534efa4b8f447172be6afcc","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5145447d2d9bb236c66f9de25a62590d5ed06620","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"150034330553741571"}
{"earner":"0x2177dee1f66d6dbfbf517d9c4f316024c6a21aeb","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"72015484794585796"}
{"earner":"0x479d40d0664b893f028b476f65ab422be7a36dca","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x358f8db7ba2dc78483863626069bcb011f9d0060","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x6a24e7fb2613e8a827dd826a87ade001d63725ee","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x355c21675662ba6d9c67b5216e7bd740801eba7e","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x7821acb8ed921e3e634ef14bb796603869bf67b5","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x45cfd19595542968b57a51e84c0e5f3c59ddb224","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xe896a22eecb31152812f5fc978a83c3b6cb1fc2d","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5daac14781a5c4af2b0673467364cba46da935db","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"209612916410360526"}
{"earner":"0xf7eaf102308678d21fa1bebaa6af31477572b893","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xb564c8a61682c4977b0fcbf4118eb1ff8ec8d803","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xc328958df2ea552d2512c5417b669a55535004a3","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xb05a4a320632e4d4c1f35f91a4874ee2bd0189b8","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xc14381e1a715a634cb2fac6865dae6b6fb943b5d","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xa5961eba49ce5d6a56ccab2ff5fbdf27f0091f0c","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x856af05d3d58bdafc91665b98aa6062abc4a97f0","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xf7dc9bf58f3200ab64d9b007baf287328c999db3","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x6b11d29a8b972535a37addf596bec76f892822f7","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xad9770d6b5514724c7b766f087bea8a784038cbe","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"54154631474851397"}
{"earner":"0x24f84af59f680e9fbf5c66ec26811016a7532c5c","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x10854d7c1580c12963c1bb4446128c6a341df3b1","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xe34ae97071d752a2b97d16e3205d906eaa76f71a","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xfb6e0d1746133e1ad5c749efc3dd789c84e88090","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x2f0a12f4e069694260b61d38e7bf64d16cc9ad60","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5c2aa9b2b1eaa5d43ab95401df3c509fc1c43ef7","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x191439c1c5b99946e89e5e737ac6f5e8809f4cf0","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x5ec90b4c142243a20dae70589a8e633a0f531609","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x4ea9a7f41dc0b74d3016bd35157043593efa0514","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"81231947212277500"}
{"earner":"0x6b09c1408d1f2a3abb3353f6a2fbc01df1a73779","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xb97bd2cc995ffdcad74e16fe3a721b20b09efd25","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x8b16a4bfb60f728d3ad8efa3ab337a13009ddbfd","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x67179ea9d048a7ba08ad7b15f9b0e015e1670795","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x0597b599d0976fc2493be0950088cc9c911c55d1","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x52033af3b4c2b7583f188bab806fa972e1f2f27a","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x155b4a1e465907861e24e6c25264be3df30071a4","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x6bc5422f8e0aa6c69731d2c72d89b21f47f287db","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xe8ded90c323af3f405c5b5461e3a3f50ce41d6a2","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x807fc1d336f2d6a48412af0c1a55663532cec929","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xac187c4278e9d015bd83dab3db5dc99434f7e765","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x1aa3e2fffe81531ca1e40c6aad6c39a10c54969b","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x2c61fd039c89bc66eb52b117ec59860f236e28af","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xf5e83919e301b961dbbbd9ad6c5fe547edec530d","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xbf1a34caabf0b22443ad8443c7ceb1756109db32","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0x3ab04e0808b10503717a938eda776842ae01e591","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"6102895758009265"}
{"earner":"0xc22e6099160c9669005c7f9f89c90947f4f2796c","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"8969408969408928490000000000"}
{"earner":"0xc22e6099160c9669005c7f9f89c90947f4f2796c","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"29387755102040715200000000000"}
{"earner":"0xba7660262c4b676fd43af18f25b5cad22cad4715","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"44081632653060808320000000000"}
{"earner":"0xc22e6099160c9669005c7f9f89c90947f4f2796c","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"19591836734693812960000000000"}
{"earner":"0x35b1a2aaaeaeb2a6d1be63454bfcadaf66348fb8","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0x08d679cf1f208684ff8334849c2ef0568cdaabe7","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"2989802989802964190000000000"}
{"earner":"0xd37f737629e0ddad7fc8adc7247d2e79c0296c35","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"4484704484704409830000000000"}
{"earner":"0xb2df8e64c8a4310edfa744158472113b6bd72fc2","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"44081632653060808320000000000"}
{"earner":"0xcca3c8a844abf1d141ea1b7d76148bcbb660aecb","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"14693877551020100240000000000"}
{"earner":"0xcca3c8a844abf1d141ea1b7d76148bcbb660aecb","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"9795918367346734000000000000"}
{"earner":"0xd358ff208a5017f96145505d1e165f30559d2781","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"5381645381645356385000000000"}
{"earner":"0x055fc8880e53d9d063f78d4fc0b8750bda2e73c6","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"8072468072468001420000000000"}
{"earner":"0x055fc8880e53d9d063f78d4fc0b8750bda2e73c6","token":"0xb6635139430eb143e1cc1d38da83ace79482ae98","snapshot":1716681600000,"cumulative_amount":"68571432"}
{"earner":"0x7db867c700a6812bf84ac4985af227c593f22a92","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0x73a855e1259d7c55b7d85facc13f0185b11caef7","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"1993201993201984220000000000"}
{"earner":"0x08d679cf1f208684ff8334849c2ef0568cdaabe7","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"6530612244897899440000000000"}
{"earner":"0x1e38e4a44ecfbd94154c1f347fbeeb74c0fcc713","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"597960597960595215000000000"}
{"earner":"0x08d679cf1f208684ff8334849c2ef0568cdaabe7","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"2607709750566869220000000000"}
{"earner":"0xb2df8e64c8a4310edfa744158472113b6bd72fc2","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"110286127983429"}
{"earner":"0xb5ab56da27ef83209ba7725f9286f58996b7066d","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"36169082275000"}
{"earner":"0xd37f737629e0ddad7fc8adc7247d2e79c0296c35","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"73524085322286"}
{"earner":"0x7db867c700a6812bf84ac4985af227c593f22a92","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"8969408969408928490000000000"}
{"earner":"0x87c83d7ae0e01cb697edab5803b7955b02524b3e","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"29387755102040715200000000000"}
{"earner":"0x098adc3bac6f2d721e4fabd90ffdf0c967501379","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"29387755102040715200000000000"}
{"earner":"0x055fc8880e53d9d063f78d4fc0b8750bda2e73c6","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"29387755102040540800000000000"}
{"earner":"0xb5ab56da27ef83209ba7725f9286f58996b7066d","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0xb004f902b643c74f5c95c2376ac748044dc310c4","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"6530612244897937920000000000"}
{"earner":"0x08d679cf1f208684ff8334849c2ef0568cdaabe7","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"1793881793881778295000000000"}
{"earner":"0x87c83d7ae0e01cb697edab5803b7955b02524b3e","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"8969408969408928490000000000"}
{"earner":"0xd358ff208a5017f96145505d1e165f30559d2781","token":"0x16b67e94257352a230636a870a4bed27b3ea9bf9","snapshot":1716681600000,"cumulative_amount":"8969408969408928490000000000"}
{"earner":"0xb2df8e64c8a4310edfa744158472113b6bd72fc2","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"29387755102040540800000000000"}
{"earner":"0xba7660262c4b676fd43af18f25b5cad22cad4715","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"29387755102040540800000000000"}
{"earner":"0xd37f737629e0ddad7fc8adc7247d2e79c0296c35","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"2690822690822645700000000000"}
{"earner":"0xb2df8e64c8a4310edfa744158472113b6bd72fc2","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"8072468072468001420000000000"}
{"earner":"0xc22e6099160c9669005c7f9f89c90947f4f2796c","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0x5f8127aaafa2bd771cadbd08c5df1570d9ed0770","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"11734693877550910190000000000"}
{"earner":"0xb004f902b643c74f5c95c2376ac748044dc310c4","token":"0x3b0d95bdf43f686242cd9c7c3aeda6ea8fe590e1","snapshot":1716681600000,"cumulative_amount":"9795918367346906400000000000"}
{"earner":"0xf58a839ba03782ea9e466d19c4c3e3c19cc3bfcb","token":"0xfc4c8b6c7a257be6e2477be98d94304978a9c0a5","snapshot":1716681600000,"cumulative_amount":"10884353741496524560000000000"}
{"earner":"0xf58a839ba03782ea9e466d19c4c3e3c19cc3bfcb","token":"0xe1b7a1249c71b538cc183b0080ffc3efd02bffb9","snapshot":1716681600000,"cumulative_amount":"2989802989802968590000000000"}
{"earner":"0x08d679cf1f208684ff8334849c2ef0568cdaabe7","token":"0xb6635139430eb143e1cc1d38da83ace79482ae98","snapshot":1716681600000,"cumulative_amount":"15238096"}
{"earner":"0x464115b4f48f676048cd685b5136bcf66f63b120","token":"0xafbbb39ef8bfb436d94902aab2746316f83a92d1","snapshot":1716681600000,"cumulative_amount":"3911564625850303830000000000"}
{"earner":"0x0d18775c9ae9fe07447995726d58e6395eed8d71","token":"0x94373a4919b3240d86ea41593d5eba789fef3848","snapshot":1716681600000,"cumulative_amount":"36169082275000"}
{"earner":"0x0d18775c9ae9fe07447995726d58e6395eed8d71","token":"0x92df4da39fe6f6c039c0c6d5da3cd481d641aaf8","snapshot":1716681600000,"cumulative_amount":"857142857142854090000000000000000000"}
{"earner":"0x5ae37715142262261052badd8c16f77db5b17ac2","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x53dd46f4c610d10b1e35ab8d0fd1de0e8a648ded","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xefb80bdd3f39c2b0e3659029c7f9b8f7ab19f4e7","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x03b51d153667725fd9373e41b26a8cfff0a2aa1a","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xf5e83919e301b961dbbbd9ad6c5fe547edec530d","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x35b1a2aaaeaeb2a6d1be63454bfcadaf66348fb8","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"5929357749999"}
{"earner":"0xcb28c4a86406ba274507d4933e64eedfca16f696","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb42fa21d25d86995d64e3eb0634008b4799f2ec0","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x1920fc4fbfdce7c9a438e266d1bc53fd63c4b665","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xcfe9c63830cb9a9944f3aa43c70265ec9bf81e6e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x392916eadfa9fa2bb95e1df9729ef7cc030e8a7e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x0b1302c23d9eb4b42a74cbefc4f9b3081ff1bf18","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"1185871550069905470"}
{"earner":"0x190e38505e13bcd85997d1282298286b037b26bf","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xfb525151815256b8992e5fe8f5b1ac3b8ecd66fa","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x1092a6b0c01b808b5277a0544b6eb2f2e4570225","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x6a24e7fb2613e8a827dd826a87ade001d63725ee","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xaac26c85853920bd64297e0aa57095eaee484667","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb38fb683657d9d0781dbd1a78119d4a1c70e4331","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb80e9e9c67d2d7bbb75e196a56c1d1c892a6c9e1","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xdeabd61ba829beb0d057038f88edcb7e64cf9242","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x5145447d2d9bb236c66f9de25a62590d5ed06620","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"3557614650208569"}
{"earner":"0xd15415c83a874d2336610b55b85724dc2c98a2d4","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x08d679cf1f208684ff8334849c2ef0568cdaabe7","token":"0x92df4da39fe6f6c039c0c6d5da3cd481d641aaf8","snapshot":1716681600000,"cumulative_amount":"285714285714284750000000000000000000"}
{"earner":"0x2177dee1f66d6dbfbf517d9c4f316024c6a21aeb","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"1245165127572855"}
{"earner":"0x8ea38052a8a0a8f45f7379e6e2f81fe52ca275b8","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xe34ae97071d752a2b97d16e3205d906eaa76f71a","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x5ec90b4c142243a20dae70589a8e633a0f531609","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x055fc8880e53d9d063f78d4fc0b8750bda2e73c6","token":"0x92df4da39fe6f6c039c0c6d5da3cd481d641aaf8","snapshot":1716681600000,"cumulative_amount":"1285714285714284560000000000000000000"}
{"earner":"0x0d18775c9ae9fe07447995726d58e6395eed8d71","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"5929357749999"}
{"earner":"0x7ab00eafc213ac2a0d61b08af68ea9951f6dac85","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x464115b4f48f676048cd685b5136bcf66f63b120","token":"0x92df4da39fe6f6c039c0c6d5da3cd481d641aaf8","snapshot":1716681600000,"cumulative_amount":"428571428571426760000000000000000000"}
{"earner":"0x358f8db7ba2dc78483863626069bcb011f9d0060","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x210f86eef811b63da42195d52e67d1896a909e48","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x0fe3f4dff1d3c7ad5497e5f76df6e33bc823e8c7","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x055fc8880e53d9d063f78d4fc0b8750bda2e73c6","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"17788073250000"}
{"earner":"0x5e4e436b7444d834733e57eacac09436215e6a2e","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x8b16a4bfb60f728d3ad8efa3ab337a13009ddbfd","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x6b09c1408d1f2a3abb3353f6a2fbc01df1a73779","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xc78b64ab536792da7b8b913f09b2954ea0b9025b","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xcca3c8a844abf1d141ea1b7d76148bcbb660aecb","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"11858715500001"}
{"earner":"0xf809ec25b04d2bde3233b9183beb9f90aa2d4249","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x38f6d15fcc481097d3c7274fc926363845b3f6af","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x7b5795fece8af52e7c70c6cebc03c0a214c72065","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x3851abcf56de870abf49db51e8402cc968a8fdd4","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xd1676b55e9c71b6964c91f5e916ca28e6b5d3369","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xcc5189e6b5ab0fad89e2cf6b2e7c2fe38cff28ce","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"11858715500001"}
{"earner":"0xea030a0dff5c5bc6043249812e391a0e6db66a0a","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x2a88d4b71a62afbe0c943f2975e3d0592d8e71a7","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x2d1ea5f682ac8ef21308551e4d641203dedc6c07","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x17c0ba040501f7c289e712bcf004d76184524eb3","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x9daf77b100b2a5c3e8e9e4f89f5df1da071e686b","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x5afa1a92750dce5a6c94ad4c2b18bc914cf0f20a","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xba7660262c4b676fd43af18f25b5cad22cad4715","token":"0x92df4da39fe6f6c039c0c6d5da3cd481d641aaf8","snapshot":1716681600000,"cumulative_amount":"1285714285714284560000000000000000000"}
{"earner":"0x15b82df5d3d8e5414c5f2627a4eb6ef86300b6ad","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x93feb4ef15c70e3fdf05aacde3e546553d063a5a","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"5929357749999"}
{"earner":"0xfc66ac6209a47a1f4fc26844fde69639925f1615","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x072fbd88688762168704431dc63eeb8335c6ed00","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x4f9c94b164143d7d2addba0791da1ad803586e68","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x6e8bc2dba7878d3676fbaf308f79ae752d95ab7d","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0x3dffe8b2371b7513b717bde49408a694f007cb6b","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
{"earner":"0xb889189803685c04a654b8c69ea494c7265598bf","token":"0xa2f77c34ec2468b902863992630b7d83e674e49a","snapshot":1716681600000,"cumulative_amount":"118587155005713"}
`
}
