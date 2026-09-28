package whatsapp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/conversation"
	"github.com/gadevsbr/tino/internal/assistente/extratos"
	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

const maxExtratoFile = 15 << 20

type stagedFile struct{ Name, Path string }
type extratoJob struct {
	ID        string       `json:"id"`
	Workbook  *stagedFile  `json:"workbook,omitempty"`
	PDFs      []stagedFile `json:"pdfs"`
	WeekSpec  string       `json:"week_spec,omitempty"`
	StartDate string       `json:"start_date,omitempty"`
	EndDate   string       `json:"end_date,omitempty"`
	WeekID    string       `json:"week_id,omitempty"`
}

var safeJobID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func (s *Service) jobDir(job extratoJob) (string, error) {
	if !safeJobID.MatchString(job.ID) {
		return "", errors.New("id de trabalho inválido")
	}
	return filepath.Join(s.cfg.DataDir, "extratos", job.ID), nil
}

func loadExtratoJob(session conversation.Session) (extratoJob, error) {
	var job extratoJob
	err := json.Unmarshal([]byte(session.Payload), &job)
	return job, err
}

func (s *Service) saveExtratoJob(ctx context.Context, session conversation.Session, job extratoJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	session.Payload = string(data)
	return conversation.NewRepository(s.domainDB).Save(ctx, session)
}

func (s *Service) handleExtratosText(ctx context.Context, sender, text string, chat types.JID) (string, bool, error) {
	n := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
	loc := s.cfg.Timezone
	if loc == nil {
		loc = time.Local
	}
	period, weekly, periodErr := extratos.ParseWeeklyCommand(text, time.Now().In(loc))
	repo := conversation.NewRepository(s.domainDB)
	session, active, err := repo.Active(ctx, sender)
	if err != nil {
		return "", true, err
	}
	if n != "extratos" && n != "processar extratos" && n != "cancelar extratos" && !weekly {
		if !active || session.Type != "EXTRATOS" {
			return "", false, nil
		}
		job, err := loadExtratoJob(session)
		if err != nil {
			return "", true, err
		}
		if job.WeekSpec == "" || job.EndDate != "" {
			return "", false, nil
		}
		p, _, err := extratos.ParseWeeklyCommand(job.WeekSpec, time.Now().In(loc))
		if err != nil {
			return "", true, err
		}
		if job.StartDate == "" {
			if _, err := p.WithDates(n, n); err != nil {
				return "❌ " + err.Error(), true, nil
			}
			job.StartDate = n
			if err := s.saveExtratoJob(ctx, session, job); err != nil {
				return "", true, err
			}
			return "Qual é a data final da semana? Responda DD/MM/AAAA.", true, nil
		}
		defined, err := p.WithDates(job.StartDate, n)
		if err != nil {
			return "❌ " + err.Error(), true, nil
		}
		job.EndDate = n
		job.WeekID = job.ID
		if err := extratos.NewRepository(s.domainDB).CreateWeek(ctx, extratos.Week{ID: job.ID, Period: defined, CreatedBy: sender}); err != nil {
			return "", true, err
		}
		if err := s.saveExtratoJob(ctx, session, job); err != nil {
			return "", true, err
		}
		return "Período definido: " + job.StartDate + " a " + job.EndDate + ". Envie os PDFs Bitz e depois `processar extratos`.", true, nil
	}
	if periodErr != nil {
		return "❌ " + periodErr.Error(), true, nil
	}
	if n == "extratos" || weekly {
		if active && session.Type == "EXTRATOS" {
			job, err := loadExtratoJob(session)
			if err != nil {
				return "", true, err
			}
			if job.WeekSpec != "" {
				if job.StartDate == "" {
					return "Qual é a data inicial dessa semana? Responda DD/MM/AAAA.", true, nil
				}
				if job.EndDate == "" {
					return "Qual é a data final dessa semana? Responda DD/MM/AAAA.", true, nil
				}
				return "Envie os PDFs Bitz da semana. Depois envie `processar extratos` ou `cancelar extratos`.", true, nil
			}
			return "Envie os PDFs Bitz e a planilha XLSX. Depois envie `processar extratos` ou `cancelar extratos`.", true, nil
		}
		if active {
			return "Há outra operação em andamento. Conclua ou cancele antes de iniciar os extratos.", true, nil
		}
		if weekly {
			stored, found, err := extratos.NewRepository(s.domainDB).FindWeek(ctx, period)
			if err != nil {
				return "", true, err
			}
			if found {
				if stored.Status != "OPEN" {
					return "Essa semana já está fechada e não aceita novos extratos.", true, nil
				}
				files, err := extratos.NewRepository(s.domainDB).Files(ctx, stored.ID)
				if err != nil {
					return "", true, err
				}
				job := extratoJob{ID: stored.ID, WeekID: stored.ID, WeekSpec: fmt.Sprintf("extrato semana %d %s %d", period.Week, extratos.MonthName(period.Month), period.Year), StartDate: stored.Period.From.Format("02/01/2006"), EndDate: stored.Period.To.Format("02/01/2006")}
				session = conversation.Session{User: sender, Type: "EXTRATOS", Status: conversation.Active}
				if err := s.saveExtratoJob(ctx, session, job); err != nil {
					return "", true, err
				}
				return fmt.Sprintf("📅 Semana retomada: %s. %d PDF(s) acumulado(s). Envie os PDFs de hoje e depois `processar extratos`; `cancelar extratos` apenas encerra o envio atual.", stored.Period.Label(), len(files)), true, nil
			}
		}
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			return "", true, err
		}
		job := extratoJob{ID: hex.EncodeToString(id[:])}
		if weekly {
			job.WeekSpec = fmt.Sprintf("extrato semana %d %s %d", period.Week, extratos.MonthName(period.Month), period.Year)
		}
		dir, err := s.jobDir(job)
		if err != nil {
			return "", true, err
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", true, err
		}
		session = conversation.Session{User: sender, Type: "EXTRATOS", Status: conversation.Active}
		if err := s.saveExtratoJob(ctx, session, job); err != nil {
			return "", true, err
		}
		if weekly {
			return fmt.Sprintf("📅 Extrato semana %d de %s/%d. Qual é a data inicial? Responda DD/MM/AAAA. Para desistir: `cancelar extratos`.", period.Week, extratos.MonthName(period.Month), period.Year), true, nil
		}
		return "📄 EXTRATOS BITZ\nEnvie um ou mais PDFs de extrato e a planilha XLSX do hotel. Depois envie `processar extratos`. Para desistir: `cancelar extratos`.\nNão envie documentos de outros hóspedes sem autorização.", true, nil
	}
	if !active || session.Type != "EXTRATOS" {
		return "Não há processamento de extratos em andamento. Envie `extratos` para começar.", true, nil
	}
	job, err := loadExtratoJob(session)
	if err != nil {
		return "", true, err
	}
	if n == "cancelar extratos" {
		session.Status = "COMPLETED"
		if err := repo.Save(ctx, session); err != nil {
			return "", true, err
		}
		if job.WeekID != "" {
			return "Envio diário encerrado. A semana e os PDFs acumulados continuam salvos; use novamente o comando da semana para retomar.", true, nil
		}
		s.cleanupExtratoJob(job)
		return "Extratos cancelados; arquivos temporários removidos.", true, nil
	}
	if job.WeekSpec != "" && (job.StartDate == "" || job.EndDate == "") {
		return "Defina primeiro as datas inicial e final da semana (DD/MM/AAAA).", true, nil
	}
	files := job.PDFs
	if job.WeekID != "" {
		stored, err := extratos.NewRepository(s.domainDB).Files(ctx, job.WeekID)
		if err != nil {
			return "", true, err
		}
		files = make([]stagedFile, 0, len(stored))
		for _, file := range stored {
			files = append(files, stagedFile{Name: file.OriginalName, Path: file.Path})
		}
	}
	if len(files) == 0 || (job.WeekSpec == "" && job.Workbook == nil) {
		if job.WeekSpec != "" {
			return "Envie pelo menos um PDF Bitz antes de processar esta semana.", true, nil
		}
		return "Envie pelo menos um PDF Bitz e uma planilha XLSX antes de processar.", true, nil
	}
	stays := make([]extratos.Stay, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file.Path)
		if err != nil {
			return "", true, err
		}
		stay, err := extratos.ParsePDF(file.Name, data)
		if err != nil {
			return fmt.Sprintf("❌ Não consegui ler %s: %v. Corrija o arquivo antes de processar.", file.Name, err), true, nil
		}
		stays = append(stays, stay)
	}
	var output, audit []byte
	var results []extratos.Result
	var outputName string
	if job.WeekSpec != "" {
		period, _, err = extratos.ParseWeeklyCommand(job.WeekSpec, time.Now().In(loc))
		if err != nil {
			return "", true, err
		}
		period, err = period.WithDates(job.StartDate, job.EndDate)
		if err != nil {
			return "❌ " + err.Error(), true, nil
		}
		output, audit, results, err = extratos.WeeklyWorkbook(period, stays)
		outputName = period.Filename()
	} else {
		workbook, readErr := os.ReadFile(job.Workbook.Path)
		if readErr != nil {
			return "", true, readErr
		}
		output, audit, results, err = extratos.Process(workbook, stays)
		outputName = "planilha_PREENCHIDA.xlsx"
	}
	if err != nil {
		return "❌ Planilha não processada: " + err.Error(), true, nil
	}
	if err := s.sendExtratoFile(ctx, chat, outputName, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", output); err != nil {
		return "", true, err
	}
	if err := s.sendExtratoFile(ctx, chat, "auditoria_extratos.json", "application/json", audit); err != nil {
		return "", true, err
	}
	session.Status = "COMPLETED"
	if err := repo.Save(ctx, session); err != nil {
		return "", true, err
	}
	if job.WeekID == "" {
		s.cleanupExtratoJob(job)
	} else if err := extratos.NewRepository(s.domainDB).MarkGenerated(ctx, job.WeekID, time.Now()); err != nil {
		return "", true, err
	}
	if job.WeekSpec != "" {
		counts := map[string]int{}
		for _, result := range results {
			counts[result.Status]++
		}
		return fmt.Sprintf("✅ %s atualizada e salva com %d PDF(s) acumulado(s). %d OK, %d CONFERIR, %d fora do período, %d duplicados. A semana continua aberta; amanhã use o mesmo comando para acrescentar novos extratos.", period.Label(), len(files), counts["OK"], counts["CONFERIR"], counts["FORA DO PERÍODO"], counts["DUPLICADO"]), true, nil
	}
	ok, review, missing := 0, 0, 0
	for _, result := range results {
		switch result.Status {
		case "OK":
			ok++
		case "CONFERIR":
			review++
		default:
			missing++
		}
	}
	return fmt.Sprintf("✅ Planilha preenchida enviada sem alterar a original. %d OK, %d CONFERIR, %d não encontrados. Revise os itens CONFERIR e o arquivo de auditoria antes do fechamento.", ok, review, missing), true, nil
}

func (s *Service) receiveExtratoDocument(ctx context.Context, sender string, doc *waE2E.DocumentMessage) (string, bool, error) {
	repo := conversation.NewRepository(s.domainDB)
	session, active, err := repo.Active(ctx, sender)
	if err != nil {
		return "", true, err
	}
	if !active || session.Type != "EXTRATOS" {
		return "", false, nil
	}
	job, err := loadExtratoJob(session)
	if err != nil {
		return "", true, err
	}
	if job.WeekSpec != "" && job.EndDate == "" {
		return "Defina primeiro as datas inicial e final da semana (DD/MM/AAAA).", true, nil
	}
	name := filepath.Base(doc.GetFileName())
	lower := strings.ToLower(name)
	isPDF, isXLSX := strings.HasSuffix(lower, ".pdf"), strings.HasSuffix(lower, ".xlsx")
	if job.WeekSpec != "" && isXLSX {
		return "Neste modo semanal, envie apenas PDFs; o bot cria a planilha automaticamente.", true, nil
	}
	if !isPDF && !isXLSX {
		return "Envie somente extratos PDF e uma planilha XLSX.", true, nil
	}
	if doc.GetFileLength() == 0 || doc.GetFileLength() > maxExtratoFile {
		return "Arquivo vazio ou maior que 15 MB; envie um arquivo menor.", true, nil
	}
	if isPDF && len(job.PDFs) >= 30 {
		return "Limite de 30 PDFs por processamento. Processe os atuais primeiro.", true, nil
	}
	data, err := s.client.Download(ctx, doc)
	if err != nil {
		return "", true, err
	}
	if len(data) == 0 || len(data) > maxExtratoFile {
		return "Arquivo vazio ou maior que 15 MB; envie um arquivo menor.", true, nil
	}
	if isPDF && !strings.HasPrefix(string(data[:min(5, len(data))]), "%PDF-") {
		return "O arquivo não é um PDF válido.", true, nil
	}
	if isXLSX && !strings.HasPrefix(string(data[:min(2, len(data))]), "PK") {
		return "O arquivo não é uma planilha XLSX válida.", true, nil
	}
	if isPDF && job.WeekID != "" {
		hash := fmt.Sprintf("%x", sha256.Sum256(data))
		dir := filepath.Join(s.cfg.DataDir, "extratos-semanais", job.WeekID)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", true, err
		}
		path := filepath.Join(dir, hash+".pdf")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return "", true, err
		}
		added, err := extratos.NewRepository(s.domainDB).AddFile(ctx, job.WeekID, name, path, hash, sender)
		if err != nil {
			_ = os.Remove(path)
			return "", true, err
		}
		files, err := extratos.NewRepository(s.domainDB).Files(ctx, job.WeekID)
		if err != nil {
			return "", true, err
		}
		if !added {
			return fmt.Sprintf("Esse PDF já estava salvo e foi ignorado. A semana continua com %d PDF(s).", len(files)), true, nil
		}
		return fmt.Sprintf("Recebido e salvo: %s. A semana agora tem %d PDF(s). Envie mais arquivos ou `processar extratos` para receber a versão atualizada.", name, len(files)), true, nil
	}
	dir, err := s.jobDir(job)
	if err != nil {
		return "", true, err
	}
	internal := fmt.Sprintf("pdf-%02d.pdf", len(job.PDFs)+1)
	if isXLSX {
		internal = "planilha.xlsx"
	}
	path := filepath.Join(dir, internal)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", true, err
	}
	if isPDF {
		job.PDFs = append(job.PDFs, stagedFile{Name: name, Path: path})
	} else {
		job.Workbook = &stagedFile{Name: name, Path: path}
	}
	if err := s.saveExtratoJob(ctx, session, job); err != nil {
		return "", true, err
	}
	return fmt.Sprintf("Recebido: %s. %d PDF(s) e planilha: %t. Envie `processar extratos` quando terminar.", name, len(job.PDFs), job.Workbook != nil), true, nil
}

func (s *Service) sendExtratoFile(ctx context.Context, chat types.JID, name, mime string, data []byte) error {
	up, err := s.client.Upload(ctx, data, whatsmeow.MediaDocument)
	if err != nil {
		return err
	}
	doc := &waE2E.DocumentMessage{URL: &up.URL, DirectPath: &up.DirectPath, MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: &up.FileLength, Mimetype: proto.String(mime), FileName: proto.String(name), Title: proto.String(name)}
	_, err = s.sendMessage(ctx, chat, &waE2E.Message{DocumentMessage: doc})
	return err
}

func (s *Service) cleanupExtratoJob(job extratoJob) {
	dir, err := s.jobDir(job)
	if err != nil {
		return
	}
	for _, file := range job.PDFs {
		if filepath.Dir(file.Path) == dir {
			_ = os.Remove(file.Path)
		}
	}
	if job.Workbook != nil && filepath.Dir(job.Workbook.Path) == dir {
		_ = os.Remove(job.Workbook.Path)
	}
	_ = os.Remove(dir)
}
