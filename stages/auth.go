package stages

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"
	"golang.org/x/crypto/acme/autocert"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// thanks to https://d3void.net/post/acme/

const contactEmail = "yourname@example.com"

func GetTLS(host, cacheDir string) (*tls.Config, error) {
	manager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(cacheDir),
		HostPolicy: autocert.HostWhitelist(host),
		Email:      contactEmail,
	}
	return &tls.Config{GetCertificate: manager.GetCertificate}, nil
}

//tls, err := GetTLS(cfg.TLSHost, cfg.TLSCacheDir)
//if err != nil {
//	return nil, errors.Wrap(err, "Could not get TLS cert")
//}
//creds := credentials.NewTLS(tls)
//server := grpc.NewServer(grpc.Creds(creds))

// return false if missing or not equals
func verifyKey(claims jwt.MapClaims, key, value string) bool {
	if i, found := claims[key]; !found {
		return false
	} else if testvalue, ok := i.(string); !ok {
		return false
	} else {
		return testvalue == value
	}
}

type userPassCred struct {
	user  string
	pass  string
	token string
}

func (a userPassCred) GetRequestMetadata(ctx context.Context, uris ...string) (map[string]string, error) {
	m := map[string]string{
		"token": a.token,
	}
	if m["token"] != "" {
		m["user"] = a.user
		m["pass"] = a.pass
	}
	return m, nil
}

func (a userPassCred) RequireTransportSecurity() bool {
	return false
}

type CognitoAuth struct {
	clientid     string
	clientsecret string
	poolid       string
	jwks_url     string
	region       string

	m map[string]rsa.PublicKey
	sync.RWMutex
}

//type CognitoAuthConfig struct {
//	ClientId     string
//	ClientSecret string
//	PoolId       string
//	JwksURL      string
//}

func NewCognitoAuth(cfg CognitoAuthConfig) (*CognitoAuth, error) {
	auth := &CognitoAuth{
		clientid:     cfg.AppClientId,
		clientsecret: cfg.AppClientSecret,
		poolid:       cfg.PoolId,
		jwks_url:     cfg.JWKUrl,
		region:       cfg.Region,
		m:            make(map[string]rsa.PublicKey),
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	go func() {
		for {
			time.Sleep(1 * time.Second)
			cognitoResp, err := client.Get(auth.jwks_url)
			if err != nil {
				log.Error(err)
				continue
				//return nil, err
			}
			defer cognitoResp.Body.Close()
			dec := json.NewDecoder(cognitoResp.Body)
			var resp cognitoKeysResponse
			if err := dec.Decode(&resp); err != nil {
				log.Error(err)
				//return nil, err
				continue
			}
			for _, key := range resp.Keys {
				n, err := base64.RawURLEncoding.DecodeString(key.N)
				if err != nil {
					log.Error("could not parse n", n)
					continue
				}
				e, err := base64.RawURLEncoding.DecodeString(key.E)
				if err != nil {
					log.Error("could not parse e", e)
					continue
				}

				var N = big.NewInt(0)
				N = N.SetBytes(n)
				var eBytes []byte
				if len(e) < 8 {
					eBytes = make([]byte, 8-len(e), 8)
					eBytes = append(eBytes, e...)
				} else {
					eBytes = e
				}

				eReader := bytes.NewReader(eBytes)
				var E uint64
				err = binary.Read(eReader, binary.BigEndian, &E)
				if err != nil {
					log.Error(err)
					continue
				}

				auth.Lock()
				auth.m[key.Kid] = rsa.PublicKey{N: N, E: int(E)}
				auth.Unlock()
			}
			time.Sleep(60 * time.Second)
		}
	}()

	return auth, nil
}

func (auth *CognitoAuth) verifyToken(tokenStr string) (refreshToken string, err error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		kid, ok := token.Header["kid"]
		if !ok {
			return nil, errors.New("no kid")
		}
		kidStr, ok := kid.(string)
		if !ok {
			return nil, errors.New("bad kid")
		}
		auth.RLock()
		defer auth.RUnlock()
		pk, found := auth.m[kidStr]
		if !found {
			return nil, nil
		}
		return &pk, nil
	})
	if err != nil {
		return "", errors.Wrapf(err, "parse jwt token err")
	}

	// How to validate
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("not good claims")
	}
	if !verifyKey(claims, "client_id", auth.clientid) {
		err = errors.New("client_id not match")
		return "", err
	}
	//	if !verifyKey(claims, "username", user) {
	//		err = errors.New(fmt.Sprintf("username not match %s", claims["username"]))
	//		return err
	//	}
	if !verifyKey(claims, "iss", strings.TrimSuffix(auth.jwks_url, "/.well-known/jwks.json")) {
		err = errors.New(fmt.Sprintf("iss not match %s", claims["iss"]))
		return "", err
	}
	if !verifyKey(claims, "token_use", "access") {
		err = errors.New("invalid token use not access")
		return "", err
	}

	if !token.Valid {
		return "", errors.New("invalid token")
	}
	return "", nil
}

func (auth *CognitoAuth) verifyUserPass(user, pass string) (accessToken string, refreshToken string, err error) {
	session := session.Must(session.NewSession())
	svc := cognitoidentityprovider.New(session, aws.NewConfig().WithRegion(auth.region))
	params := make(map[string]*string)
	params["USERNAME"] = &user
	params["PASSWORD"] = &pass

	sig := hmac.New(sha256.New, []byte(auth.clientsecret))
	sig.Write([]byte(user + auth.clientid))
	secret_hash := base64.StdEncoding.EncodeToString(sig.Sum(nil))
	params["SECRET_HASH"] = &secret_hash

	req := &cognitoidentityprovider.AdminInitiateAuthInput{}
	req = req.SetAuthFlow("ADMIN_NO_SRP_AUTH").
		SetAuthParameters(params).
		SetClientId(auth.clientid).
		SetUserPoolId(auth.poolid)
	if validateErr := req.Validate(); validateErr != nil {
		return "", "", errors.Wrap(validateErr, "got validation error")
	}

	output, err := svc.AdminInitiateAuth(req)
	if err != nil {
		return "", "", errors.Wrap(err, "initiate auth err")
	}

	accessToken = *output.AuthenticationResult.AccessToken
	refreshToken = *output.AuthenticationResult.RefreshToken
	_, err = auth.verifyToken(accessToken)
	return accessToken, refreshToken, err
}

type cognitoKeysResponse struct {
	Keys []cognitoKey `json:"keys"`
}

type cognitoKey struct {
	Alg string `json:"alg"`
	E   string `json:"e"`
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	Use string `json:"use"`
}
