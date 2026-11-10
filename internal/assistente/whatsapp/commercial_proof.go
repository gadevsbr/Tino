package whatsapp

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gadevsbr/tino/internal/assistente/bitz"
	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

const maxCommercialProofBytes = 15 << 20

type commercialProof struct {
	ID, Account, Contact, ChatJID, MediaType, FileName string
	Data                                               []byte
}

func (s *Service) handleCommercialProof(ctx context.Context, account, contact, messageID string, chat types.JID, image *waE2E.ImageMessage, document *waE2E.DocumentMessage) (bool, error) {
	waiting, err := s.commercial.AwaitingPaymentProof(ctx, account, contact)
	if err != nil || !waiting || (image == nil && document == nil) {
		return false, err
	}
	request, err := s.commercial.PendingPreReservation(ctx, account, contact)
	if err != nil || request == nil {
		return true, errors.New("seleção da hospedagem incompleta")
	}
	var downloadable whatsmeow.DownloadableMessage
	proof := commercialProof{ID: transportMessageKey("proof", account, contact, messageID), Account: account, Contact: contact, ChatJID: chat.ToNonAD().String()}
	if image != nil {
		downloadable = image
		proof.MediaType = image.GetMimetype()
		if proof.MediaType == "" {
			proof.MediaType = "image/jpeg"
		}
		proof.FileName = "comprovante.jpg"
	} else {
		if document.GetMimetype() != "application/pdf" && !strings.HasSuffix(strings.ToLower(document.GetFileName()), ".pdf") {
			return false, nil
		}
		downloadable = document
		proof.MediaType, proof.FileName = "application/pdf", document.GetFileName()
		if proof.FileName == "" {
			proof.FileName = "comprovante.pdf"
		}
	}
	if s.downloadOverride != nil {
		proof.Data, err = s.downloadOverride(ctx, downloadable)
	} else {
		proof.Data, err = s.client.Download(ctx, downloadable)
	}
	if err != nil {
		return true, err
	}
	if len(proof.Data) == 0 || len(proof.Data) > maxCommercialProofBytes {
		return true, errors.New("comprovante vazio ou acima de 15 MB")
	}
	if proof.MediaType == "application/pdf" && !strings.HasPrefix(string(proof.Data[:min(5, len(proof.Data))]), "%PDF-") {
		return true, errors.New("PDF de comprovante inválido")
	}
	if strings.HasPrefix(proof.MediaType, "image/") && !strings.HasPrefix(http.DetectContentType(proof.Data), "image/") {
		return true, errors.New("imagem de comprovante inválida")
	}
	_, err = s.domainDB.ExecContext(ctx, `INSERT OR IGNORE INTO commercial_payment_proofs(id,account,contact,chat_jid,media_type,file_name,data,created_at) VALUES(?,?,?,?,?,?,?,?)`, proof.ID, account, contact, proof.ChatJID, proof.MediaType, proof.FileName, proof.Data, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return true, err
	}
	notice := proofValidationNotice(contact, *request)
	destinations := append([]string(nil), s.auth.Numbers()...)
	if cfg, cfgErr := s.bitzStore.Public(ctx); cfgErr == nil && cfg.ApproverPhone != "" {
		destinations = append(destinations, cfg.ApproverPhone)
	}
	sent := map[string]bool{}
	delivered := 0
	for _, phone := range destinations {
		phone, ok := phoneArgument(phone)
		if !ok || sent[phone] {
			continue
		}
		sent[phone] = true
		if err := s.sendProofToPhone(ctx, phone, notice, proof); err != nil {
			s.commercial.RecordFailure(ctx, "whatsapp", account, contact, err)
		} else {
			delivered++
		}
	}
	if delivered == 0 {
		return true, errors.New("nenhum operador recebeu o comprovante")
	}
	if err := s.commercial.PaymentProofDelivered(ctx, account, contact); err != nil {
		return true, err
	}
	s.startPreReservation(account, contact, messageID, chat, *request)
	s.replyText(ctx, chat, "Recebi seu comprovante e encaminhei para validação. Nossa equipe confirmará a reserva por aqui.")
	return true, nil
}

func proofValidationNotice(contact string, request commercial.PreReservation) string {
	return "🧾 *Comprovante aguardando validação*\n\n" + reservationSummary(contact, request) + "\n\nO arquivo recebido está anexado a esta mensagem."
}

func (s *Service) sendProofToPhone(ctx context.Context, phone, caption string, proof commercialProof) error {
	lookup, err := s.client.IsOnWhatsApp(ctx, []string{"+" + phone})
	if err != nil || len(lookup) != 1 || !lookup[0].IsIn || lookup[0].JID.IsEmpty() {
		return errors.New("operador não encontrado no WhatsApp")
	}
	return s.sendCommercialProof(ctx, lookup[0].JID, caption, proof)
}

func (s *Service) sendCommercialProof(ctx context.Context, to types.JID, caption string, proof commercialProof) error {
	mediaType := whatsmeow.MediaDocument
	if strings.HasPrefix(proof.MediaType, "image/") {
		mediaType = whatsmeow.MediaImage
	}
	var up whatsmeow.UploadResponse
	var err error
	if s.uploadOverride != nil {
		up, err = s.uploadOverride(ctx, proof.Data, mediaType)
	} else {
		up, err = s.client.Upload(ctx, proof.Data, mediaType)
	}
	if err != nil {
		return err
	}
	if mediaType == whatsmeow.MediaImage {
		message := &waE2E.ImageMessage{URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: proto.Uint64(up.FileLength), Mimetype: proto.String(proof.MediaType), Caption: proto.String(caption)}
		_, err = s.sendMessage(ctx, to, &waE2E.Message{ImageMessage: message})
		return err
	}
	document := &waE2E.DocumentMessage{URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath), MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: proto.Uint64(up.FileLength), Mimetype: proto.String(proof.MediaType), FileName: proto.String(proof.FileName), Title: proto.String(proof.FileName), Caption: proto.String(caption)}
	_, err = s.sendMessage(ctx, to, &waE2E.Message{DocumentMessage: document})
	return err
}

func (s *Service) latestCommercialProof(ctx context.Context, job bitz.Job) (commercialProof, error) {
	var proof commercialProof
	err := s.domainDB.QueryRowContext(ctx, `SELECT id,account,contact,chat_jid,media_type,file_name,data FROM commercial_payment_proofs WHERE account=? AND contact=? ORDER BY created_at DESC LIMIT 1`, job.Account, job.Contact).Scan(&proof.ID, &proof.Account, &proof.Contact, &proof.ChatJID, &proof.MediaType, &proof.FileName, &proof.Data)
	if errors.Is(err, sql.ErrNoRows) {
		return proof, nil
	}
	return proof, err
}
