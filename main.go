package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
	"github.com/thom151/leadme/internal/auth"
	"github.com/thom151/leadme/internal/database"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	secret         string
	elevenApiKey   string
	openaiApiKey   string
	assistantID    string
	twAuthToken    string
	twAccountSid   string
	ngrokUrl       string
	openaiClient   *openai.Client
	deepgramApiKey string
	deepgramConn   *websocket.Conn
	s3Bucket       string
	s3Region       string
	s3CfDistro     string
	s3Client       *s3.Client
	mu             sync.Mutex
	heygenApiKey   string
}

func main() {

	const filepathRoot = "./templates/"
	const port = "8080"

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Cannot load env" + err.Error())
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DB url not set")
	}

	platform := os.Getenv("PLATFORM")
	if platform == "" {
		log.Fatal("Platform not set")
	}

	secret := os.Getenv("SECRET")
	if secret == "" {
		log.Fatal("secret not set")
	}

	elevenApiKey := os.Getenv("EL_API_KEY")
	if elevenApiKey == "" {
		log.Fatal("eleven api key not working")
	}

	openaiApiKey := os.Getenv("OPENAI_API_KEY")
	if openaiApiKey == "" {
		log.Fatal("openai key not working")
	}

	assistantID := os.Getenv("ASSISTANT_ID")
	if assistantID == "" {
		log.Fatal("assistant id not working")
	}

	twAuthToken := os.Getenv("TWILIO_AUTH_TOKEN")
	if twAuthToken == "" {
		log.Fatal("twilio auth token not set")
	}

	twAccountSid := os.Getenv("TWILIO_ACCOUNT_SID")
	if twAccountSid == "" {
		log.Fatal("twilio accound sid not set")
	}

	ngrokUrl := os.Getenv("NGROK_URL")
	if ngrokUrl == "" {
		log.Fatal("ngrok url not set")
	}

	deepgramApiKey := os.Getenv("DEEPGRAM_API_KEY")
	if deepgramApiKey == "" {
		log.Fatal("deepgram api key not set")
	}

	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		log.Fatal("s3 bucket not provided")
	}

	s3Region := os.Getenv("AWS_REGION")
	if s3Region == "" {
		log.Fatal("s3 region not set")
	}

	s3CfDistro := os.Getenv("S3_CF_DISTRO")
	if s3CfDistro == "" {
		log.Fatal("s3 cf distro not set")
	}

	heygenApiKey := os.Getenv("HEYGEN_API_KEY")
	if twAuthToken == "" {
		log.Fatal("heygen api key not set")
	}

	db, err := sql.Open("libsql", dbURL)
	if err != nil {
		log.Fatal("Cannot open db" + err.Error())
	}

	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatal("Cannot load aws default config", err)
	}

	s3Client := s3.NewFromConfig(awsConfig)

	dbQueries := database.New(db)
	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		db:             dbQueries,
		platform:       platform,
		secret:         secret,
		elevenApiKey:   elevenApiKey,
		openaiApiKey:   openaiApiKey,
		assistantID:    assistantID,
		ngrokUrl:       ngrokUrl,
		s3Client:       s3Client,
		s3Bucket:       s3Bucket,
		s3Region:       s3Region,
		s3CfDistro:     s3CfDistro,
		openaiClient:   openai.NewClient(openaiApiKey),
		deepgramApiKey: deepgramApiKey,
		heygenApiKey:   heygenApiKey,
	}
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	//	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(filepathRoot)))))

	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)

	mux.HandleFunc("/api/users", apiCfg.handlerUsersCreate)
	mux.HandleFunc("/api/login", apiCfg.handlerLogin)
	mux.HandleFunc("/api/clone-voice", apiCfg.handlerCloneVoice)
	mux.HandleFunc("/api/speak", apiCfg.handlerSpeak)
	mux.HandleFunc("/app/", apiCfg.handlerIndex)

	mux.HandleFunc("/api/call", apiCfg.handleCall)
	mux.HandleFunc("/api/twiml", apiCfg.handleTwiml)
	mux.HandleFunc("/api/stream", apiCfg.handleStream)
	mux.HandleFunc("POST /api/refresh", apiCfg.handlerRefresh)
	mux.HandleFunc("POST /api/revoke", apiCfg.handlerRevoke)

	mux.HandleFunc("POST /api/video_upload/{videoID}", apiCfg.handlerUploadVideo)
	mux.HandleFunc("/api/videos", apiCfg.handlerVideoMetaCreate)
	mux.HandleFunc("POST /api/video_series_meta", apiCfg.handlerVideoSeriesMetaCreate)
	mux.HandleFunc("GET /api/video_series", apiCfg.handlerCreateVideoSeries)
	mux.HandleFunc("POST /api/video_series", apiCfg.handlerUploadMp3ToS3)
	mux.HandleFunc("POST /api/create_client", apiCfg.handlerCreateClient)
	mux.HandleFunc("GET /api/record", apiCfg.handlerRecordVideo)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	log.Fatal(srv.ListenAndServe())
}

func (cfg *apiConfig) handlerIndex(w http.ResponseWriter, r *http.Request) {
	type AssistandData struct {
		VoiceAssistants []database.VoiceAssistant
	}

	token, err := auth.GetBearerToken(r.Header, r.Cookies())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "couldn't find tokens", err.Error())
		return
	}

	userId, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "invalid tokens", err.Error())
		return
	}
	voice_assistants, err := cfg.db.GetAssistantsByUserID(r.Context(), userId.String())

	for _, voice_assistant := range voice_assistants {
		fmt.Println(voice_assistant.ClonedVoiceID)
	}

	voiceAssistantData := AssistandData{
		VoiceAssistants: voice_assistants,
	}
	renderTemplate(w, "home", voiceAssistantData)
}
