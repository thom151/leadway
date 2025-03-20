package main

import (
	//	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thom151/leadme/internal/database"

	"fmt"

	"github.com/twilio/twilio-go"
	api "github.com/twilio/twilio-go/rest/api/v2010"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type DeepgramResp struct {
	Channel struct {
		Alternatives []struct {
			Transcript string  `json:"transcript"`
			Confidence float64 `json:"confidence"`
			Words      []struct {
				Word           string  `json:"word"`
				Start          float64 `json:"start"`
				End            float64 `json:"end"`
				Confidence     float64 `json:"confidence"`
				PunctuatedWord string  `json:"punctuated_word"`
			} `json:"words"`
		} `json:"alternatives"`
	} `json:"channel"`
	Type        string  `json:"type"`
	Start       float64 `json:"start"`
	IsFinal     bool    `json:"is_final"`
	SpeechFinal bool    `json:"speech_final"`
}

func (cfg *apiConfig) handleCall(w http.ResponseWriter, r *http.Request) {
	user, err := validateAndReturnUser(r, cfg)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "unautorized access", err.Error())
		return
	}
	openAIc := cfg.openaiClient
	//get from

	thread, err := cfg.db.GetThread(r.Context(), database.GetThreadParams{
		UserID:    user.ID,
		ContactID: "0413810844",
	})
	if err != nil {
		threadID, err := genThread(openAIc, user.Username)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "error generating thread", err.Error())
			return
		}

		thread, err = cfg.db.CreateThread(r.Context(), database.CreateThreadParams{
			UserID:    user.ID,
			ContactID: "0413810844",
			ThreadID:  threadID.ID,
		})
	}

	client := twilio.NewRestClient()
	params := &api.CreateCallParams{}
	params.SetUrl("https" + cfg.ngrokUrl + "api/twiml?threadID=" + thread.ThreadID)

	params.SetTo("+61413810844")
	params.SetFrom("+61375005604")

	_, err = client.Api.CreateCall(params)
	if err != nil {
		fmt.Println(err.Error())
		respondWithError(w, http.StatusInternalServerError, "error creating call", err.Error())
		return
	}
}

func (cfg *apiConfig) handleTwiml(w http.ResponseWriter, r *http.Request) {

	threadID := r.URL.Query().Get("threadID")
	if threadID == "" {
		http.Error(w, "thread id missing", http.StatusBadRequest)
		return
	}

	twiml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
        <Response>
            <Connect>
				<Stream url="wss%sapi/stream">
                    <Parameter name="threadID" value="%s"/>
                </Stream>
            </Connect>
        </Response>`, cfg.ngrokUrl, threadID)
	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(twiml))
}

func (cfg *apiConfig) handleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading request to WebSocket:", err)
		http.Error(w, "Failed to upgrade to WebSocket", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	done := make(chan struct{})

	threadID, streamSid := extractThreadID(conn)

	startTime := time.Now()
	dgConn, err := createDeepgramConnection(cfg.deepgramApiKey)
	if err != nil {
		log.Println("error connecting to deepgram", err)
		return
	}
	log.Println("call setup took :", time.Since(startTime))
	defer dgConn.Close()

	xiConn, err := createElevenConnection(cfg, "HGfmRCkOadeBGOFiuKZW")
	if err != nil {
		log.Println("error connecting to elevenlabs", err)
		return
	}
	defer xiConn.Close()

	audioChan := make(chan []byte, 10)

	var lastSent time.Time
	go sendAudioToChannel(dgConn, conn, done, &lastSent)
	go processDeepgramResp(dgConn, xiConn, conn, done, threadID, streamSid, cfg, audioChan, &lastSent)
	go sendTextToElevenLabs(audioChan, xiConn)

	log.Println("Starting conversation...")

	<-done

}

func sendAudioToChannel(deepgramConn *websocket.Conn, conn *websocket.Conn, done chan struct{}, lastSent *time.Time) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket closed or error reading message:", err)
			close(done)
			return
		}

		*lastSent = time.Now()
		audioData := processMessage(message)
		if audioData != nil {
			if err := deepgramConn.WriteMessage(websocket.BinaryMessage, audioData); err != nil {
				log.Println("error sending audio to deepgram: ", err)
				return
			}
		}

	}
}

func processDeepgramResp(deepgramConn, xiConn, conn *websocket.Conn, done chan struct{}, threadID, streamSid string, cfg *apiConfig, audioChan chan []byte, lastSent *time.Time) {
	for {
		select {
		case <-done:
			return
		default:
		}
		_, message, err := deepgramConn.ReadMessage()
		if err != nil {
			log.Println("error receiving deepgram response: ", err)
			return
		}

		var dgResp DeepgramResp
		err = json.Unmarshal(message, &dgResp)
		if err != nil {
			log.Println("error unmarhsaling deepgram response: ", err)
			return
		}

		if dgResp.Type == "Results" && len(dgResp.Channel.Alternatives) > 0 {
			alt := dgResp.Channel.Alternatives[0]
			if alt.Transcript != "" && dgResp.IsFinal {
				log.Printf("Transcript: %s, Latency: %v", alt.Transcript, time.Since(*lastSent))
				chunkChan := make(chan string, 10)

				log.Println("sending to openai ... ")
				aiResp, err := streamOpenAIResponse(cfg, threadID, alt.Transcript, chunkChan)
				if err != nil {
					log.Println("error getting open ai reponse: ", err)
					continue
				}
				log.Println("Full response: ", aiResp)

				/*	textMsg := map[string]interface{}{
						"text":                   aiResp,
						"try_trigger_generation": false,
					}

					err = xiConn.WriteJSON(textMsg)
					if err != nil {
						log.Println("error generating audio", err)
						return
					}

					for audio := range audioChan {
						log.Println("received audio chunk")
						err = sendAudioToTwilio(conn, audio, streamSid)
						if err != nil {
							log.Println("error sending to twilio", err)
							return
						}
					}
				*/
			}

		}

	}
}

func sendTextToElevenLabs(audioChan chan []byte, xiConn *websocket.Conn) {
	defer close(audioChan)
	for {
		var msg map[string]interface{}
		err := xiConn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				log.Println("elevenlabs websocket close normally")
				return
			}
			log.Println("error reading from elevenlabs:", err)
			return
		}

		if audioData, ok := msg["audio"].(string); ok {
			audioBytes, err := base64.StdEncoding.DecodeString(audioData)
			if err != nil {
				log.Println("error decoding audio chunk:", err)
				continue
			}
			log.Println("sending audio to channel")
			audioChan <- audioBytes
		} else {
			log.Println("failed getting audio")
		}
	}
}

func processMessage(message []byte) []byte {
	var data struct {
		Media struct {
			Payload string `json:"payload"`
		} `json:"media"`
	}
	if err := json.Unmarshal(message, &data); err != nil {
		log.Println("error unmarshalling message: ", err)
		return nil
	}
	audio, err := base64.StdEncoding.DecodeString(data.Media.Payload)
	if err != nil {
		log.Println("failed to decode audio: ", err)
		return nil
	}
	return audio
}

func extractThreadID(conn *websocket.Conn) (string, string) {

	var twilioEvent struct {
		Event     string `json:"event"`
		StreamSid string `json:"streamSid"`
		Start     struct {
			CustomParameters map[string]string `json:"customParameters"`
		} `json:"start"`
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("error reading message")
			return "", ""
		}

		if err := json.Unmarshal(msg, &twilioEvent); err != nil {
			log.Println("Error parsing Twilio JSON:", err)
			return "", ""
		}

		if twilioEvent.Event == "start" {
			if twilioEvent.Start.CustomParameters != nil {
				if threadID, ok := twilioEvent.Start.CustomParameters["threadID"]; ok {
					return threadID, twilioEvent.StreamSid
				}
			}
		}

	}
}
