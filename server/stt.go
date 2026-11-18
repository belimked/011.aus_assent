package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"time"
)

var sttPhrases = []string{"Sunnybank", "Sunnybank Hills", "Macgregor", "Kessels Rd", "Mains Rd", "Runcorn", "Calamvale", "Eight Mile Plains", "Robertson", "Coopers Plains", "Gager St", "Rex", "open home", "appraisal"}

// Overridden only by local httptest servers in tests.
var azureFastBaseURL = ""

func wavDuration(b []byte) (*int64, bool) {
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, false
	}
	var byteRate, dataSize uint32
	for i := 12; i+8 <= len(b); {
		size := binary.LittleEndian.Uint32(b[i+4 : i+8])
		start := i + 8
		if string(b[i:i+4]) == "data" {
			dataSize = size
			break
		}
		if uint64(start)+uint64(size) > uint64(len(b)) {
			break
		}
		switch string(b[i : i+4]) {
		case "fmt ":
			if size >= 16 {
				byteRate = binary.LittleEndian.Uint32(b[start+8 : start+12])
			}
		}
		i = start + int(size) + int(size&1)
	}
	if byteRate == 0 || dataSize == 0 {
		return nil, true
	}
	ms := int64(dataSize) * 1000 / int64(byteRate)
	return &ms, true
}

func (a *app) transcribe(w http.ResponseWriter, r *http.Request, p principal) {
	started := time.Now()
	meta := map[string]any{"bytes": 0, "latencyMs": 0}
	defer func() { meta["latencyMs"] = time.Since(started).Milliseconds(); a.auditCall(p, "stt", meta) }()
	if os.Getenv("AXLEO_AZURE_SPEECH_KEY") == "" {
		problem(w, 503, "stt not configured")
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
		problem(w, 400, "invalid multipart input")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	defer r.Body.Close()
	mr, err := r.MultipartReader()
	if err != nil {
		problem(w, 400, "invalid multipart input")
		return
	}
	var audio []byte
	lang := "zh"
	filename := "audio.wav"
	for {
		part, e := mr.NextPart()
		if e == io.EOF {
			break
		}
		if e != nil {
			problem(w, 400, "invalid multipart input")
			return
		}
		switch part.FormName() {
		case "audio":
			if audio != nil || part.FileName() == "" {
				part.Close()
				problem(w, 400, "invalid audio")
				return
			}
			filename = part.FileName()
			audio, e = io.ReadAll(part)
		case "lang":
			var b []byte
			b, e = io.ReadAll(io.LimitReader(part, 16))
			lang = string(b)
		default:
			_, e = io.Copy(io.Discard, part)
		}
		part.Close()
		if e != nil {
			problem(w, 400, "invalid multipart input")
			return
		}
	}
	if len(audio) == 0 || (lang != "en" && lang != "zh" && lang != "yue") {
		problem(w, 400, "invalid audio or lang")
		return
	}
	meta["bytes"] = len(audio)
	meta["lang"] = lang
	duration, wav := wavDuration(audio)
	if duration != nil && *duration > 120000 {
		problem(w, 400, "audio exceeds 120 seconds")
		return
	}
	extension := strings.ToLower(filename)
	mediaType := ""
	switch {
	case strings.HasSuffix(extension, ".wav"):
		mediaType = "audio/wav"
		if !wav {
			problem(w, 400, "invalid WAV")
			return
		}
	case strings.HasSuffix(extension, ".m4a"):
		mediaType = "audio/mp4"
	case strings.HasSuffix(extension, ".mp3"):
		mediaType = "audio/mpeg"
	case strings.HasSuffix(extension, ".webm"):
		mediaType = "audio/webm"
	default:
		problem(w, 400, "unsupported audio type")
		return
	}
	// Never forward the user's original filename to a third party.
	filename = "audio" + extension[strings.LastIndex(extension, "."):]
	locales := map[string][]string{"en": {"en-AU"}, "zh": {"zh-CN"}, "yue": {"zh-HK"}}
	url := azureFastBaseURL
	if url == "" {
		url = "https://" + env("AXLEO_AZURE_SPEECH_REGION", "australiaeast") + ".api.cognitive.microsoft.com"
	}
	url += "/speechtotext/transcriptions:transcribe?api-version=2025-10-15" // phraseList needs >= 2025-10-15
	for attempt := 0; attempt < 2; attempt++ {
		definition := map[string]any{"locales": locales[lang]}
		if attempt == 0 {
			definition["phraseList"] = map[string]any{"phrases": sttPhrases}
		}
		var payload bytes.Buffer
		mw := multipart.NewWriter(&payload)
		headers := textproto.MIMEHeader{}
		headers.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "audio", "filename": filename}))
		headers.Set("Content-Type", mediaType)
		file, _ := mw.CreatePart(headers)
		_, _ = file.Write(audio)
		b, _ := json.Marshal(definition)
		_ = mw.WriteField("definition", string(b))
		_ = mw.Close()
		req, e := http.NewRequestWithContext(r.Context(), "POST", url, &payload)
		if e != nil {
			problem(w, 502, "stt upstream error")
			return
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Ocp-Apim-Subscription-Key", os.Getenv("AXLEO_AZURE_SPEECH_KEY"))
		resp, e := speechHTTPClient.Do(req)
		if e != nil {
			log.Printf("stt upstream request failed: %v", e)
			problem(w, 502, "stt upstream error")
			return
		}
		if resp.StatusCode != 200 {
			log.Printf("stt upstream status %d (attempt %d)", resp.StatusCode, attempt)
		}
		if resp.StatusCode == 400 && attempt == 0 {
			resp.Body.Close()
			continue
		}
		if resp.StatusCode == 429 {
			w.Header().Set("Retry-After", resp.Header.Get("Retry-After"))
			resp.Body.Close()
			problem(w, 429, "stt rate limited")
			return
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			problem(w, 502, "stt upstream error")
			return
		}
		var result struct {
			CombinedPhrases []struct {
				Text string `json:"text"`
			} `json:"combinedPhrases"`
			DurationMilliseconds *int64 `json:"durationMilliseconds"`
		}
		e = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result)
		resp.Body.Close()
		if e != nil {
			problem(w, 502, "stt upstream error")
			return
		}
		texts := make([]string, 0, len(result.CombinedPhrases))
		for _, phrase := range result.CombinedPhrases {
			texts = append(texts, phrase.Text)
		}
		if result.DurationMilliseconds != nil {
			duration = result.DurationMilliseconds
		}
		respond(w, 200, map[string]any{"text": strings.Join(texts, " "), "lang": lang, "durationMs": duration, "latencyMs": time.Since(started).Milliseconds()})
		return
	}
}

var speechHTTPClient = &http.Client{Timeout: 45 * time.Second}
