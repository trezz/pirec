package pirec

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
)

func TrimSilence(src, dst string) error {
	cmd := exec.Command("sox", src, dst, "silence", "-l", "1", "0", "0.2%", "-1", "1.0", "0.2%")
	return cmd.Run()
}

func SpeechToText(src string, azLocation, azToken, lang string) (string, error) {
	reader, err := os.Open(src)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://%v.stt.speech.microsoft.com/speech/recognition/conversation/cognitiveservices/v1?language=%v",
		azLocation, lang)
	req, err := http.NewRequest("POST", url, reader)
	if err != nil {
		return "", err
	}

	req.Header.Add("Ocp-Apim-Subscription-Key", azToken)
	req.Header.Add("Content-Type", "audio/wav")

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		return "", fmt.Errorf("%v", res)
	}

	respData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	resp := map[string]interface{}{}
	err = json.Unmarshal(respData, &resp)

	text := resp["DisplayText"].(string)

	return text, nil
}
