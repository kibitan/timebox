package server

import (
	"fmt"

	"banking/model"

	"github.com/kode4food/timebox"
	"github.com/kode4food/timebox/command"
	"github.com/kode4food/timebox/event"
	"github.com/kode4food/timebox/store"
)

// Error messages
const (
	errAccountExists = "account already exists: %s"
	errOverdraft     = "withdrawal would result in overdraft"
)

// Command types
const (
	OpenAccount   = "open-account"
	DepositMoney  = "deposit-money"
	WithdrawMoney = "withdraw-money"
)

// OpenAccountCommandWithID attaches a newly generated ID to the Command
type OpenAccountCommandWithID struct {
	model.OpenAccountCommand
	model.AccountID
}

// Handler returns a new Handler for Account-related Commands
func Handler(es *event.Source) command.Handler {
	th := command.TypedHandler{
		OpenAccount:   makeOpenAccount(es),
		DepositMoney:  makeDepositMoney(es),
		WithdrawMoney: makeWithdrawMoney(es),
	}
	return th.Handler()
}

func makeOpenAccount(es *event.Source) command.Handler {
	return func(c *timebox.Command) error {
		p := c.Payload.(OpenAccountCommandWithID)
		return es.With(
			p.AccountID,
			func(a *event.Aggregate, result store.Result) error {
				if result.NextVersion() != store.InitialVersion {
					return fmt.Errorf(errAccountExists, p.AccountID)
				}
				a.Raise(event.New(
					model.AccountOpened,
					&model.AccountOpenedEvent{
						AccountID: p.AccountID,
						Owner:     p.Owner,
					},
				))
				return nil
			},
		)
	}
}

func makeDepositMoney(es *event.Source) command.Handler {
	return func(c *command.Command) error {
		p := c.Payload.(model.MoneyTransferCommand)
		if err := p.Check(); err != nil {
			return err
		}

		return es.With(
			p.AccountID,
			func(a *event.Aggregate, result store.Result) error {
				if _, err := model.HydrateFrom(a, result); err != nil {
					return err
				}
				a.Raise(event.New(
					model.MoneyDeposited,
					&model.MoneyDepositedEvent{
						AccountID:       p.AccountID,
						DepositedAmount: p.Amount,
					},
				))
				return nil
			},
		)
	}
}

func makeWithdrawMoney(es *event.Source) command.Handler {
	return func(c *command.Command) error {
		p := c.Payload.(model.MoneyTransferCommand)
		if err := p.Check(); err != nil {
			return err
		}

		return es.With(
			p.AccountID,
			func(a *event.Aggregate, result store.Result) error {
				if acc, err := model.HydrateFrom(a, result); err != nil {
					return err
				} else {
					a.Raise(event.New(
						model.MoneyWithdrawn,
						&model.MoneyWithdrawnEvent{
							AccountID:       p.AccountID,
							WithdrawnAmount: p.Amount,
						},
					))
					if acc.Balance.IsNegative() {
						return fmt.Errorf(errOverdraft)
					}
				}
				return nil
			},
		)
	}
}
