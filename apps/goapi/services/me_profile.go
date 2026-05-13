package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"goapi/internal/events"
	"goapi/logger"
	"goapi/models"
	"goapi/repositories"
)

func defaultSettingsModel(userID uuid.UUID) models.UserSettings {
	return models.UserSettings{
		UserID:                    userID,
		Theme:                     "system",
		NotificationPreferences:   json.RawMessage(`{}`),
		ExtraSettings:             json.RawMessage(`{}`),
		MarketingEmailOptIn:       false,
		SecurityNotificationOptIn: true,
	}
}

func hasMePatch(input *PatchMeProfileInput) bool {
	if input == nil {
		return false
	}
	return input.FirstName != nil || input.LastName != nil ||
		input.DisplayName != nil || input.AvatarURL != nil ||
		input.PhoneNum != nil || input.Timezone != nil || input.Locale != nil ||
		input.Theme != nil || input.NotificationPreferences != nil ||
		input.MarketingEmailOptIn != nil || input.SecurityNotificationOptIn != nil ||
		input.ExtraSettings != nil
}

// GetMyProfile returns the authenticated user's profile plus settings (self-only via actor UUID).
func (s *userService) GetMyProfile(ctx context.Context, actorID uuid.UUID) (*MeProfileDTO, error) {
	if actorID == uuid.Nil {
		return nil, ErrInsufficientPrivileges
	}
	user, err := s.userRepo.FindByID(actorID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user for profile: %w", err)
	}
	settings, err := s.settingsRepo.FindByUserID(actorID)
	switch {
	case errors.Is(err, repositories.ErrUserSettingsNotFound):
		d := defaultSettingsModel(actorID)
		return &MeProfileDTO{User: user, Settings: d}, nil
	case err != nil:
		return nil, fmt.Errorf("get settings: %w", err)
	default:
		return &MeProfileDTO{User: user, Settings: *settings}, nil
	}
}

// PatchMyProfile applies partial profile/settings updates for the authenticated user only.
func (s *userService) PatchMyProfile(ctx context.Context, actorID uuid.UUID, input *PatchMeProfileInput) (*MeProfileDTO, error) {
	if actorID == uuid.Nil {
		return nil, ErrInsufficientPrivileges
	}
	if !hasMePatch(input) {
		return nil, ErrNoFieldsToUpdate
	}

	user, err := s.userRepo.FindByID(actorID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("load user for patch: %w", err)
	}

	st := defaultSettingsModel(actorID)
	if existing, err := s.settingsRepo.FindByUserID(actorID); err == nil {
		st = *existing
	} else if !errors.Is(err, repositories.ErrUserSettingsNotFound) {
		return nil, fmt.Errorf("load settings: %w", err)
	}

	if input.FirstName != nil {
		fn, err := SanitizeString(*input.FirstName, maxNameLen)
		if err != nil || fn == "" {
			return nil, ErrInvalidProfileField
		}
		user.FirstName = fn
	}
	if input.LastName != nil {
		ln, err := SanitizeString(*input.LastName, maxNameLen)
		if err != nil || ln == "" {
			return nil, ErrInvalidProfileField
		}
		user.LastName = ln
	}
	if input.DisplayName != nil {
		dn, err := SanitizeString(*input.DisplayName, maxDisplayNameLen)
		if err != nil {
			return nil, ErrInvalidProfileField
		}
		user.DisplayName = dn
	}
	if input.AvatarURL != nil {
		v := strings.TrimSpace(*input.AvatarURL)
		if v == "" {
			user.AvatarURL = ""
		} else {
			if err := ValidateHTTPSURL(v, maxAvatarURLLen); err != nil {
				return nil, ErrInvalidAvatarURL
			}
			user.AvatarURL = v
		}
	}
	if input.PhoneNum != nil {
		p, err := SanitizeString(*input.PhoneNum, maxPhoneLen)
		if err != nil {
			return nil, ErrInvalidProfileField
		}
		user.PhoneNum = p
	}
	if input.Timezone != nil {
		tz := strings.TrimSpace(*input.Timezone)
		if tz == "" {
			user.Timezone = ""
		} else {
			if err := ValidateIANATimezone(tz); err != nil {
				return nil, ErrInvalidTimezone
			}
			user.Timezone = tz
		}
	}
	if input.Locale != nil {
		loc := strings.TrimSpace(*input.Locale)
		if loc == "" {
			user.Locale = ""
		} else {
			if err := ValidateLocaleFormat(loc); err != nil {
				return nil, ErrInvalidLocale
			}
			user.Locale = loc
		}
	}

	if input.Theme != nil {
		t := strings.TrimSpace(strings.ToLower(*input.Theme))
		if _, ok := ValidThemes[t]; !ok {
			return nil, ErrInvalidTheme
		}
		st.Theme = t
	}
	if input.NotificationPreferences != nil {
		b := []byte(*input.NotificationPreferences)
		if err := ValidateJSONObjectBytes(b); err != nil {
			return nil, ErrInvalidNotificationPrefs
		}
		st.NotificationPreferences = json.RawMessage(append([]byte(nil), b...))
	}
	if input.MarketingEmailOptIn != nil {
		st.MarketingEmailOptIn = *input.MarketingEmailOptIn
	}
	if input.SecurityNotificationOptIn != nil {
		st.SecurityNotificationOptIn = *input.SecurityNotificationOptIn
	}
	if input.ExtraSettings != nil {
		b := []byte(*input.ExtraSettings)
		if err := ValidateJSONObjectBytes(b); err != nil {
			return nil, ErrInvalidExtraSettingsJSON
		}
		st.ExtraSettings = json.RawMessage(append([]byte(nil), b...))
	}

	if err := s.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("update user profile: %w", err)
	}
	if err := s.settingsRepo.Upsert(&st); err != nil {
		return nil, fmt.Errorf("upsert settings: %w", err)
	}

	s.cache.DeleteUser(ctx, user.ID, user.Email)

	events.RecordSafe(s.recorder, ctx, events.Event{
		OccurredAt:  events.NowUTC(),
		EventType:   "user.profile.updated",
		ActorUserID: actorPtrFromCtx(ctx),
		Subject:     actorID.String(),
		Metadata:    events.MetadataJSON(map[string]string{"user_id": actorID.String()}),
		RequestID:   events.RequestIDFromContext(ctx),
	})

	out, err := s.GetMyProfile(ctx, actorID)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("reload profile after patch")
		return &MeProfileDTO{User: user, Settings: st}, nil
	}
	return out, nil
}
