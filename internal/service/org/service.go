// Package org implements the use cases around sender organizations: CRUD from
// the settings page, plus seeding the customer's default firm on startup.
package org

import (
	"context"
	"errors"
	"time"

	domain "github.com/FIFSAK/saubala-back/internal/domain/org"
	"github.com/FIFSAK/saubala-back/internal/domain/release"
	"github.com/FIFSAK/saubala-back/pkg/store"
	"github.com/FIFSAK/saubala-back/pkg/web"
)

// Service manages the sender organizations.
type Service struct {
	orgs     domain.Repository
	releases release.Repository
}

func NewService(orgs domain.Repository, releases release.Repository) *Service {
	return &Service{orgs: orgs, releases: releases}
}

// Input carries the editable fields of an organization.
type Input struct {
	Name                 string
	BIN                  string
	ResponsibleForSupply string
	Director             string
	Accountant           string
}

func (s *Service) Create(ctx context.Context, in Input) (*domain.Organization, error) {
	o, err := domain.New(in.Name, in.BIN, in.ResponsibleForSupply, in.Director, in.Accountant)
	if err != nil {
		return nil, web.BadRequest(err.Error())
	}
	if err := s.orgs.Create(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Organization, error) {
	o, err := s.orgs.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.NotFound("организация не найдена")
		}
		return nil, err
	}
	return o, nil
}

func (s *Service) List(ctx context.Context) ([]domain.Organization, error) {
	return s.orgs.List(ctx)
}

func (s *Service) Update(ctx context.Context, id string, in Input) (*domain.Organization, error) {
	o, err := s.orgs.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.NotFound("организация не найдена")
		}
		return nil, err
	}
	o.Name = in.Name
	o.BIN = in.BIN
	o.ResponsibleForSupply = in.ResponsibleForSupply
	o.Director = in.Director
	o.Accountant = in.Accountant
	o.Normalize()
	if err := o.Validate(); err != nil {
		return nil, web.BadRequest(err.Error())
	}
	o.UpdatedAt = time.Now().UTC()
	if err := s.orgs.Update(ctx, o); err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return nil, web.NotFound("организация не найдена")
		}
		return nil, err
	}
	return o, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.orgs.GetByID(ctx, id); err != nil {
		if errors.Is(err, store.ErrorNotFound) {
			return web.NotFound("организация не найдена")
		}
		return err
	}
	count, err := s.releases.CountByOrganization(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return web.Conflict("организация используется в отгрузках, её нельзя удалить")
	}
	return s.orgs.Delete(ctx, id)
}

// EnsureDefault seeds the customer's current firm as the first organization if
// none exist yet. It is idempotent and safe to run on every startup.
func (s *Service) EnsureDefault(ctx context.Context) error {
	count, err := s.orgs.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return s.orgs.Create(ctx, domain.Default())
}
