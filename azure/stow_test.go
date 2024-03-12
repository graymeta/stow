package azure

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/cheekybits/is"
	"github.com/flyteorg/stow"
	"github.com/flyteorg/stow/test"
)

var (
	azureaccount         = os.Getenv("AZUREACCOUNT")
	azurekey             = os.Getenv("AZUREKEY")
	azureBigFileTestSize = os.Getenv("AZUREBIGFILETESTSIZEMB")
)

func presignedRequestPreparer(method stow.ClientMethod, r *http.Request) error {
	if method == stow.ClientMethodPut {
		r.Header.Set("x-ms-blob-type", "BlockBlob")
	}
	return nil
}

func TestStowWithSharedKeyAuth(t *testing.T) {
	if azureaccount == "" {
		t.Skip("skipping test because missing AZUREACCOUNT")
	}
	if azurekey == "" {
		t.Skip("skipping test because missing AZUREKEY")
	}

	cfg := stow.ConfigMap{"account": azureaccount, "key": azurekey}
	test.All(t, "azure", cfg)
	test.ContainerPreSignRequest(t, "azure", cfg, presignedRequestPreparer)
	test.ExistingContainerDoesNotProduceAnError(t, "azure", cfg)
}

func TestStowWithDefaultADAuth(t *testing.T) {
	if azureaccount == "" {
		t.Skip("skipping test because missing AZUREACCOUNT")
	}

	cfg := stow.ConfigMap{"account": azureaccount}
	test.All(t, "azure", cfg)
	test.ContainerPreSignRequest(t, "azure", cfg, presignedRequestPreparer)
	test.ExistingContainerDoesNotProduceAnError(t, "azure", cfg)
}

func TestBigFileUpload(t *testing.T) {
	if azureBigFileTestSize == "" {
		t.Skip("skipping test because missing AZUREBIGFILETESTSIZEMB is not set")
	}
	if azureaccount == "" {
		t.Skip("skipping test because missing AZUREACCOUNT")
	}

	fileSize, err := strconv.ParseInt(azureBigFileTestSize, 10, 64)
	if err != nil {
		t.Fatalf("Invalid value for AZUREBIGFILETESTSIZEMB: %s", azureBigFileTestSize)
	}

	cfg := stow.ConfigMap{"account": azureaccount}
	test.BigFileUpload(t, "azure", cfg, fileSize*1000*1000)
}

func TestEtagCleanup(t *testing.T) {
	etagValue := "9c51403a2255f766891a1382288dece4"
	permutations := []string{
		`"%s"`,       // Enclosing quotations
		`W/\"%s\"`,   // Weak tag identifier with escapted quotes
		`W/"%s"`,     // Weak tag identifier with quotes
		`"\"%s"\"`,   // Double quotes, inner escaped
		`""%s""`,     // Double quotes,
		`"W/"%s""`,   // Double quotes with weak identifier
		`"W/\"%s\""`, // Double quotes with weak identifier, inner escaped
	}
	for index, p := range permutations {
		testStr := fmt.Sprintf(p, etagValue)
		cleanTestStr := cleanEtag(testStr)
		if etagValue != cleanTestStr {
			t.Errorf(`Failure at permutation #%d (%s), result: %s`,
				index, permutations[index], cleanTestStr)
		}
	}
}

func TestMetaMapRoundTrip(t *testing.T) {
	is := is.New(t)

	stowMap := make(map[string]interface{})
	stowMap["one"] = "two"
	stowMap["3"] = "4"
	stowMap["ninety-nine"] = "100"

	expectedAzureMap := make(map[string]*string)
	for key, value := range stowMap {
		vStr, _ := value.(string)
		expectedAzureMap[key] = &vStr
	}

	//returns map[string]interface
	azureCompatMap, err := makeAzureCompatMetadataMap(stowMap)
	is.NoErr(err)

	if !reflect.DeepEqual(azureCompatMap, expectedAzureMap) {
		t.Errorf("Expected map (%+v) and returned map (%+v) are not equal.", expectedAzureMap, azureCompatMap)
	}

	convertedStowMap := makeStowCompatMetadataMap(azureCompatMap)
	if !reflect.DeepEqual(stowMap, convertedStowMap) {
		t.Errorf("Expected map (%+v) and returned map (%+v) are not equal.", stowMap, convertedStowMap)
	}
}

func TestMakeAzureCompatMetadataMapFailureWithNonStringValues(t *testing.T) {
	is := is.New(t)

	m := make(map[string]interface{})
	m["float"] = 8.9
	m["number"] = 9

	_, err := makeAzureCompatMetadataMap(m)
	is.Err(err)
}

func TestRemovedConfigOptionsCausesFailure(t *testing.T) {
	is := is.New(t)
	for _, removedKey := range removedConfigKeys {
		cfg := stow.ConfigMap{"account": "ignore"}
		cfg[removedKey] = "anything"
		err := stow.Validate("azure", cfg)
		is.Err(err)
		if !strings.Contains(err.Error(), "removed config option used") ||
			!strings.Contains(err.Error(), removedKey) {
			is.Failf("Unexpected error message: %s", err.Error())
		}

	}
}
