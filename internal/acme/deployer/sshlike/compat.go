package sshlike

import "github.com/LemonZuo/homer/internal/model"

func ToSSHTarget(t Target) *model.ACMESSHTarget {
	return &model.ACMESSHTarget{
		ID:              t.ID,
		Name:            t.Name,
		Host:            t.Host,
		Port:            t.Port,
		AuthSource:      t.AuthSource,
		CredentialID:    t.CredentialID,
		BastionTargetID: t.BastionTargetID,
		Username:        t.Username,
		AuthType:        t.AuthType,
		Password:        t.Password,
		PrivateKey:      t.PrivateKey,
		Passphrase:      t.Passphrase,
		Enabled:         t.Enabled,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

func FromSSHTarget(t *model.ACMESSHTarget) *Target {
	return &Target{
		ID:              t.ID,
		Name:            t.Name,
		Host:            t.Host,
		Port:            t.Port,
		AuthSource:      t.AuthSource,
		CredentialID:    t.CredentialID,
		BastionTargetID: t.BastionTargetID,
		Username:        t.Username,
		AuthType:        t.AuthType,
		Password:        t.Password,
		PrivateKey:      t.PrivateKey,
		Passphrase:      t.Passphrase,
		Enabled:         t.Enabled,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

func ApplyToSSHTarget(src *Target, dst *model.ACMESSHTarget) {
	dst.ID = src.ID
	dst.Name = src.Name
	dst.Host = src.Host
	dst.Port = src.Port
	dst.AuthSource = src.AuthSource
	dst.CredentialID = src.CredentialID
	dst.BastionTargetID = src.BastionTargetID
	dst.Username = src.Username
	dst.AuthType = src.AuthType
	dst.Password = src.Password
	dst.PrivateKey = src.PrivateKey
	dst.Passphrase = src.Passphrase
	dst.Enabled = src.Enabled
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}
