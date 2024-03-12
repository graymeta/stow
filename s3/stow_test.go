// build +disabled
package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/cheekybits/is"
	"github.com/flyteorg/stow"
	"github.com/flyteorg/stow/test"
)

func TestStow(t *testing.T) {
	accessKeyId := os.Getenv("S3ACCESSKEYID")
	secretKey := os.Getenv("S3SECRETKEY")
	region := os.Getenv("S3REGION")

	if accessKeyId == "" || secretKey == "" || region == "" {
		t.Skip("skipping test because missing one or more of S3ACCESSKEYID S3SECRETKEY S3REGION")
	}

	config := stow.ConfigMap{
		"access_key_id": accessKeyId,
		"secret_key":    secretKey,
		"region":        region,
	}

	test.All(t, "s3", config)
}

func TestPreSignedURL(t *testing.T) {
	is := is.New(t)
	accessKeyId := os.Getenv("S3ACCESSKEYID")
	secretKey := os.Getenv("S3SECRETKEY")
	region := os.Getenv("S3REGION")

	if accessKeyId == "" || secretKey == "" || region == "" {
		t.Skip("skipping test because missing one or more of S3ACCESSKEYID S3SECRETKEY S3REGION")
	}

	config := stow.ConfigMap{
		"access_key_id": accessKeyId,
		"secret_key":    secretKey,
		"region":        region,
	}

	location, err := stow.Dial("s3", config)
	is.NoErr(err)

	container, err := location.Container("flyte-demo")
	ctx := context.Background()
	res, err := container.PreSignRequest(ctx, stow.ClientMethodPut, "blah/bloh/fileon", stow.PresignRequestParams{
		ExpiresIn: time.Hour,
	})

	is.NoErr(err)
	t.Log(res)
	assert.NotEmpty(t, res)
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

func TestPrepMetadataSuccess(t *testing.T) {
	is := is.New(t)

	m := make(map[string]*string)
	m["one"] = aws.String("two")
	m["3"] = aws.String("4")
	m["ninety-nine"] = aws.String("100")

	m2 := make(map[string]interface{})
	for key, value := range m {
		str := *value
		m2[key] = str
	}

	returnedMap, err := prepMetadata(m2)
	is.NoErr(err)

	if !reflect.DeepEqual(m, returnedMap) {
		t.Error("Expected and returned maps are not equal.")
	}
}

func TestPrepMetadataFailureWithNonStringValues(t *testing.T) {
	is := is.New(t)

	m := make(map[string]interface{})
	m["float"] = 8.9
	m["number"] = 9

	_, err := prepMetadata(m)
	is.Err(err)
}

func TestInvalidAuthtype(t *testing.T) {
	is := is.New(t)

	config := stow.ConfigMap{
		"auth_type": "foo",
	}
	_, err := stow.Dial("s3", config)
	is.Err(err)
	is.True(strings.Contains(err.Error(), "invalid auth_type"))
}

func TestV2SigningEnabled(t *testing.T) {
	is := is.New(t)

	//check v2 singing occurs
	v2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		is.True(strings.HasPrefix(r.Header.Get("Authorization"), "AWS access-key:"))
		w.Header().Add("ETag", "something")
		w.WriteHeader(http.StatusOK)
	}))
	defer v2Server.Close()

	uri, err := url.Parse(v2Server.URL + "/testing")
	is.NoErr(err)

	config := stow.ConfigMap{
		"access_key_id": "access-key",
		"secret_key":    "secret-key",
		"region":        "do-not-care",
		"v2_signing":    "true",
		"endpoint":      v2Server.URL,
	}

	location, err := stow.Dial("s3", config)
	is.NoErr(err)
	_, _ = location.ItemByURL(uri)

	//check v2 signing does not occur
	v4Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		is.False(strings.HasPrefix(r.Header.Get("Authorization"), "AWS access-key:"))
		w.Header().Add("ETag", "something")
		w.WriteHeader(http.StatusOK)
	}))
	defer v4Server.Close()

	uri, err = url.Parse(v4Server.URL + "/testing")
	is.NoErr(err)

	config = stow.ConfigMap{
		"access_key_id": "access-key",
		"secret_key":    "secret-key",
		"region":        "do-not-care",
		"v2_signing":    "false",
		"endpoint":      v4Server.URL,
	}

	location, err = stow.Dial("s3", config)
	is.NoErr(err)
	_, _ = location.ItemByURL(uri)
}

func TestWillNotRequestRegionWhenConfigured(t *testing.T) {
	is := is.New(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		is.Fail("Request should not occur")
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	config := stow.ConfigMap{
		"access_key_id": "access-key",
		"secret_key":    "secret-key",
		"region":        "do-not-care",
		"endpoint":      server.URL,
	}

	location, err := stow.Dial("s3", config)
	is.NoErr(err)

	_, err = location.Container("Whatever")

	is.NoErr(err)
}

func TestWillRequestRegionWhenConfigured(t *testing.T) {
	is := is.New(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		awsLocationQuery, err := url.ParseQuery("location")
		is.NoErr(err)
		is.Equal(awsLocationQuery.Encode(), r.URL.RawQuery)
		b, _ := json.Marshal(s3.GetBucketLocationOutput{
			LocationConstraint: aws.String("whatever"),
		})
		w.Write(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := stow.ConfigMap{
		"access_key_id": "access-key",
		"secret_key":    "secret-key",
		"endpoint":      server.URL,
	}

	location, err := stow.Dial("s3", config)
	is.NoErr(err)

	_, err = location.Container("Whatever")

	// Make sure that this is an error
	is.NoErr(err)
}
