package keys

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var nominalData = []struct {
	pssh    string
	url     string
	headers map[string]string
	keys    []string
}{
	{
		"AAAAV3Bzc2gAAAAA7e+LqXnWSs6jyCfc1R0h7QAAADcIARIQ62dqu8s0Xpa7z2FmMPGj2hoNd2lkZXZpbmVfdGVzdCIQZmtqM2xqYVNkZmFsa3IzajIA",
		"https://cwip-shaka-proxy.appspot.com/no_auth",
		nil,
		[]string{
			"ccbf5fb4c2965be7aa130ffb3ba9fd73:9cc0c92044cb1d69433f5f5839a159df",
			"9bf0e9cf0d7b55aeb4b289a63bab8610:90f52fd8ca48717b21d0c2fed7a12ae1",
			"eb676abbcb345e96bbcf616630f1a3da:100b6c20940f779a4589152b57d2dacb",
			"0294b9599d755de2bbf0fdca3fa5eab7:3bda2f40344c7def614227b9c0f03e26",
			"639da80cf23b55f3b8cab3f64cfa5df6:229f5f29b643e203004b30c4eaf348f4",
		},
	},
	{
		"AAAAVHBzc2gAAAAA7e+LqXnWSs6jyCfc1R0h7QAAADQIARIQS1wsIy3vDgJa68dVktqRoRoJYmVsbG1lZGlhIhNmZi1jNTQzMmIwZC0xMzM0Mjky",
		"https://license.9c9media.ca/widevine",
		nil,
		[]string{
			"4b5c2c232def0e025aebc75592da91a1:b975cb999ef82c2617230f8a36b53047",
			"cf03357caeb21472ae071266f4f6f2ed:70feca835ddb8b3742c3340ceb7fa78b",
			"a06c0be58a414b36115b0fc47902ed2c:c52e7a34050d9217e0f7a21d34a9a0be",
		},
	},
	// {
	// 	"AAAANHBzc2gAAAAA7e+LqXnWSs6jyCfc1R0h7QAAABQIARIQFTDToGkERGqRoTOhFaqMQQ==",
	// 	"https://drm-widevine-licensing.axtest.net/AcquireLicense",
	// 	nil,
	// 	[]string{
	// 		"1e52b7bfac7c27d3c5ccba3507dcfcc9:798cbae7fa6e8e03f5284f1ce1155d15",
	// 		"439447312cb0527eb53d078fdd4054e1:798cbae7fa6e8e03f5284f1ce1155d15",
	// 		"adbd00254fd25162a37462603bf2d08a:798cbae7fa6e8e03f5284f1ce1155d15",
	// 	},
	// },
}

var wrongData = []struct {
	pssh    string
	url     string
	headers map[string]string
	keys    []string
}{
	{
		"AAAAVHBzc2gAAAAA7e+LqXnWSs6jyCfc1R0h7QAAADcIARIQ62dqu8sASD0Xpa7z2FmMPGj2hoNd2lDASkZXZpbmVfdGVzdCIQZmtqM2xqYVNkZmFsaADS3IzajIA",
		"https://cwip-shaka-proxy.appspot.com/no_auth",
		nil,
		[]string{},
	},
	{
		"AAAAVHBzc2gAAAAA7e+LqXnWSs6jyCfc1R0h7QAAADQIARIQS1wsIy3vDgJa68dVktqRoRoJYmVsbG1lZGlhIhNmZi1jNTQzMmIwZC0xMzM0Mjky",
		"https://license.9c9media.ca/playready",
		nil,
		[]string{},
	},
	{
		"AAAAVHBzc2gAAAAA7e+LqXnWSs6jyCfc1R0h7QAAADQIARIQSASD1wsIy3vDgJa68dVkASDtqRoRoJYmVsbG1lZGlhIhASDNmZi1jNTQzMmIwZC0xMASDzM0Mjky", // random stuff
		"https://cwip-shaka-proxy.appspot.com/no_auth",
		nil,
		[]string{},
	},
}

/*
Level: Unit
Strategy: Exploring strategy
Characteristic: Functional
*/
func TestGetWidevineDecryptionKeys(t *testing.T) {
	for _, d := range nominalData {
		keys, err := Get(d.pssh, d.url, d.headers)

		require.Nil(t, err, err)
		require.NotEmpty(t, keys, "no widevine decryption keys generated")
		require.ElementsMatch(t, d.keys, keys, "widevine keys don't match expected ones")
	}
}

/*
Level: Unit
Strategy: Exploring strategy
Characteristic: Functional
*/
func TestGetWidevineDecryptionKeysWithWrongInformations(t *testing.T) {
	for _, d := range wrongData {
		keys, err := Get(d.pssh, d.url, d.headers)

		require.Error(t, err, "widevine decryption keys not returning error with bad data entry")
		require.Nil(t, keys, "widevine decryption keys returning keys with bad data entry")
	}
}
