package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"goapi/internal/email"
	"goapi/internal/queue"
	queuejobs "goapi/internal/queue/jobs"
	"goapi/internal/rbac"
	"goapi/logger"
	"goapi/models"
	"goapi/repositories"
	"gorm.io/gorm"
)

// CreateOrganizationInvitationInput is the body for POST /organizations/:id/invitations.
type CreateOrganizationInvitationInput struct {
	Email string
	Role  string
}

// OrganizationInvitationDTO is a non-secret invite row (no token).
type OrganizationInvitationDTO struct {
	ID              uuid.UUID `json:"id"`
	OrganizationID  uuid.UUID `json:"organization_id"`
	Email           string    `json:"email"`
	Role            string    `json:"role"`
	InvitedByUserID uuid.UUID `json:"invited_by_user_id"`
	ExpiresAt       string    `json:"expires_at"`
	AcceptedAt      *string   `json:"accepted_at,omitempty"`
	CreatedAt       string    `json:"created_at"`
}

func invitationToDTO(i *models.OrganizationInvitation) *OrganizationInvitationDTO {
	d := &OrganizationInvitationDTO{
		ID:              i.ID,
		OrganizationID:  i.OrganizationID,
		Email:           i.Email,
		Role:            i.Role,
		InvitedByUserID: i.InvitedByUserID,
		ExpiresAt:       i.ExpiresAt.UTC().Format(time.RFC3339),
		CreatedAt:       i.CreatedAt.UTC().Format(time.RFC3339),
	}
	if i.AcceptedAt != nil {
		s := i.AcceptedAt.UTC().Format(time.RFC3339)
		d.AcceptedAt = &s
	}
	return d
}

func generateInvitationRawToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func invitationTokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// CreateOrganizationInvitation issues a single-use invite; supersedes other pending invites for the same email.
func (s *organizationService) CreateOrganizationInvitation(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID, input *CreateOrganizationInvitationInput) (*OrganizationInvitationDTO, error) {
	if input == nil {
		return nil, ErrInvalidInvitationRole
	}
	emailAddr := normalizeEmail(input.Email)
	if emailAddr == "" || !strings.Contains(emailAddr, "@") {
		return nil, ErrInvalidInvitationEmail
	}
	role := strings.TrimSpace(strings.ToLower(input.Role))
	if !rbac.IsValidOrgInviteRole(role) {
		return nil, ErrInvalidInvitationRole
	}

	org, err := s.orgRepo.FindByID(orgID)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationNotFound) {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}
	orgName := org.Name

	actorMem, err := s.orgRepo.FindMembership(orgID, actorID)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationMembershipNotFound) {
			return nil, ErrInsufficientPrivileges
		}
		return nil, err
	}
	if !rbac.OrgRoleCanManageOrganization(actorMem.Role) {
		return nil, ErrInsufficientPrivileges
	}

	u, uErr := s.userRepo.FindByEmail(emailAddr)
	if uErr == nil && u != nil {
		if _, mErr := s.orgRepo.FindMembership(orgID, u.ID); mErr == nil {
			return nil, ErrAlreadyOrganizationMember
		} else if !errors.Is(mErr, repositories.ErrOrganizationMembershipNotFound) {
			return nil, mErr
		}
	} else if uErr != nil && !errors.Is(uErr, repositories.ErrUserNotFound) {
		return nil, uErr
	}

	if err := s.orgRepo.DeletePendingInvitationsForEmail(orgID, emailAddr); err != nil {
		return nil, err
	}

	rawToken, err := generateInvitationRawToken()
	if err != nil {
		return nil, err
	}
	hash := invitationTokenHash(rawToken)

	ttlHours := 168
	if s.cfg != nil && s.cfg.Email.OrganizationInvitationTTLHours > 0 {
		ttlHours = s.cfg.Email.OrganizationInvitationTTLHours
	}
	expires := time.Now().UTC().Add(time.Duration(ttlHours) * time.Hour)

	inv := &models.OrganizationInvitation{
		OrganizationID:  orgID,
		Email:           emailAddr,
		Role:            role,
		TokenHash:       hash,
		InvitedByUserID: actorID,
		ExpiresAt:       expires,
	}
	if err := s.orgRepo.CreateInvitation(inv); err != nil {
		if isLikelyUniqueViolation(err) {
			return nil, ErrInvalidOrganizationInvitation
		}
		return nil, err
	}

	s.emitOrgWebhook(ctx, orgID, WebhookEventOrganizationInvitationCreated, map[string]interface{}{
		"invitation_id": inv.ID.String(),
		"email":         inv.Email,
		"role":          inv.Role,
	})

	s.dispatchOrgInvitationEmail(ctx, emailAddr, rawToken, orgName)

	return invitationToDTO(inv), nil
}

func (s *organizationService) dispatchOrgInvitationEmail(ctx context.Context, to, rawToken, orgName string) {
	if s.cfg == nil {
		return
	}
	// When async queue is enabled, publish to workers; otherwise send inline (sync).
	if s.cfg.Queue.Enabled && s.jobQueue != nil {
		payload, err := json.Marshal(struct {
			To               string `json:"to"`
			RawToken         string `json:"raw_token"`
			OrganizationName string `json:"organization_name"`
		}{To: to, RawToken: rawToken, OrganizationName: orgName})
		if err != nil {
			logger.Log.Warn().Err(err).Msg("org invitation email: marshal failed")
			s.sendOrgInvitationEmailSync(ctx, to, rawToken, orgName)
			return
		}
		if err := s.jobQueue.Enqueue(ctx, queuejobs.EmailSendOrganizationInvitation, payload, queue.EnqueueOptions{}); err != nil {
			logger.Log.Warn().Err(err).Msg("org invitation email: enqueue failed; sending inline")
			s.sendOrgInvitationEmailSync(ctx, to, rawToken, orgName)
		}
		return
	}
	s.sendOrgInvitationEmailSync(ctx, to, rawToken, orgName)
}

func (s *organizationService) sendOrgInvitationEmailSync(ctx context.Context, to, rawToken, orgName string) {
	if s.mail == nil || s.cfg == nil {
		return
	}
	msg := email.NewOrganizationInvitationMessage(s.cfg, to, rawToken, orgName)
	if err := s.mail.Send(ctx, msg); err != nil {
		logger.Log.Warn().Err(err).Msg("org invitation email: send failed")
	}
}

// ListOrganizationInvitations returns pending and historical invites (admin/owner only).
func (s *organizationService) ListOrganizationInvitations(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID) ([]OrganizationInvitationDTO, error) {
	if _, err := s.orgRepo.FindByID(orgID); err != nil {
		if errors.Is(err, repositories.ErrOrganizationNotFound) {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}
	actorMem, err := s.orgRepo.FindMembership(orgID, actorID)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationMembershipNotFound) {
			return nil, ErrInsufficientPrivileges
		}
		return nil, err
	}
	if !rbac.OrgRoleCanManageOrganization(actorMem.Role) {
		return nil, ErrInsufficientPrivileges
	}

	rows, err := s.orgRepo.ListInvitationsByOrg(orgID)
	if err != nil {
		return nil, err
	}
	out := make([]OrganizationInvitationDTO, 0, len(rows))
	for i := range rows {
		out = append(out, *invitationToDTO(&rows[i]))
	}
	return out, nil
}

// AcceptOrganizationInvitation consumes a token and adds (or upgrades) membership when the account email matches.
func (s *organizationService) AcceptOrganizationInvitation(ctx context.Context, actorID uuid.UUID, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || s.db == nil {
		return ErrInvalidOrganizationInvitation
	}

	user, err := s.userRepo.FindByID(actorID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	userEmail := normalizeEmail(user.Email)

	hash := invitationTokenHash(rawToken)

	var emitOrgID uuid.UUID
	var emitInvID uuid.UUID
	var emitRole string

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inv models.OrganizationInvitation
		if err := tx.Where("token_hash = ?", hash).First(&inv).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalidOrganizationInvitation
			}
			return err
		}

		now := time.Now().UTC()
		if inv.AcceptedAt != nil {
			return ErrOrganizationInvitationAccepted
		}
		if now.After(inv.ExpiresAt) {
			return ErrOrganizationInvitationExpired
		}

		if normalizeEmail(inv.Email) != userEmail {
			return ErrOrganizationInvitationEmailMismatch
		}

		emitOrgID = inv.OrganizationID
		emitInvID = inv.ID
		emitRole = inv.Role

		var existing models.OrganizationMembership
		err := tx.Where("organization_id = ? AND user_id = ?", inv.OrganizationID, actorID).First(&existing).Error
		if err == nil {
			// Already a member: accept invite idempotently (mark invite consumed).
			if existing.Role == rbac.OrgRoleOwner && inv.Role != rbac.OrgRoleOwner {
				// keep owner
			} else if rbac.OrgInviteRank(inv.Role) > rbac.OrgInviteRank(existing.Role) {
				existing.Role = inv.Role
				existing.UpdatedAt = now
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			mem := models.OrganizationMembership{
				OrganizationID: inv.OrganizationID,
				UserID:         actorID,
				Role:           inv.Role,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err := tx.Create(&mem).Error; err != nil {
				return err
			}
		} else {
			return err
		}

		return tx.Model(&models.OrganizationInvitation{}).
			Where("id = ?", inv.ID).
			Updates(map[string]interface{}{
				"accepted_at": now,
				"updated_at":  now,
			}).Error
	})
	if err != nil {
		return err
	}
	s.emitOrgWebhook(ctx, emitOrgID, WebhookEventOrganizationInvitationAccepted, map[string]interface{}{
		"invitation_id": emitInvID.String(),
		"user_id":       actorID.String(),
		"role":          emitRole,
	})
	return nil
}

// RemoveOrganizationMember removes a user from the organization (owner/admin only); blocks removing the last owner.
func (s *organizationService) RemoveOrganizationMember(ctx context.Context, orgID uuid.UUID, actorID uuid.UUID, targetUserID uuid.UUID) error {
	if _, err := s.orgRepo.FindByID(orgID); err != nil {
		if errors.Is(err, repositories.ErrOrganizationNotFound) {
			return ErrOrganizationNotFound
		}
		return err
	}

	actorMem, err := s.orgRepo.FindMembership(orgID, actorID)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationMembershipNotFound) {
			return ErrInsufficientPrivileges
		}
		return err
	}
	if !rbac.OrgRoleCanManageOrganization(actorMem.Role) {
		return ErrInsufficientPrivileges
	}

	targetMem, err := s.orgRepo.FindMembership(orgID, targetUserID)
	if err != nil {
		if errors.Is(err, repositories.ErrOrganizationMembershipNotFound) {
			return ErrNotOrganizationMember
		}
		return err
	}

	if targetMem.Role == rbac.OrgRoleOwner {
		n, err := s.orgRepo.CountMembersWithRole(orgID, rbac.OrgRoleOwner)
		if err != nil {
			return err
		}
		if n <= 1 {
			return ErrCannotRemoveLastOwner
		}
	}

	if err := s.orgRepo.DeleteMembership(orgID, targetUserID); err != nil {
		return err
	}
	s.emitOrgWebhook(ctx, orgID, WebhookEventOrganizationMemberRemoved, map[string]interface{}{
		"user_id": targetUserID.String(),
	})
	return nil
}
