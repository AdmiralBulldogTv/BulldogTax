package utils

import (
	"fmt"

	"github.com/dgrijalva/jwt-go"
	jsoniter "github.com/json-iterator/go"
	"github.com/troydota/bulldog-taxes/configure"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const alg = `{"alg": "HS256","typ": "JWT"}`

func SignJWT(pl interface{}) (string, error) {
	bytes, err := json.MarshalToString(pl)
	if err != nil {
		return "", err
	}

	algEnc := jwt.EncodeSegment(S2B(alg))
	payload := jwt.EncodeSegment(S2B(bytes))

	first := fmt.Sprintf("%s.%s", algEnc, payload)

	sign, err := jwt.SigningMethodHS256.Sign(first, S2B(configure.Config.GetString("jwt_secret")))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.%s", first, sign), nil
}

func VerifyJWT(token []string, output interface{}) error {
	if len(token) != 3 {
		return fmt.Errorf("invalid token")
	}
	if err := jwt.SigningMethodHS256.Verify(fmt.Sprintf("%s.%s", token[0], token[1]), token[2], S2B(configure.Config.GetString("jwt_secret"))); err != nil {
		return err
	}

	val, err := jwt.DecodeSegment(token[1])
	if err != nil {
		return err
	}

	return json.Unmarshal(val, output)
}
