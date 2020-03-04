package main

import (
	"fmt"
)

// chanCreationError is an enum which describes the exact nature of an error
// encountered when a user attempts to create a channel with the faucet. This
// enum is used within the templates to determine at which input item the error
// occurred  and also to generate an error string unique to the error.
type ChanCreationError uint8

const (
	// NoError is the default error which indicates either the form hasn't
	// yet been submitted or no errors have arisen.
	NoError ChanCreationError = iota

	// InvalidAddress indicates that the passed node address is invalid.
	InvalidAddress

	// NotConnected indicates that the target peer isn't connected to the
	// faucet.
	NotConnected

	// ChanAmountNotNumber indicates that the amount specified for the
	// amount to fund the channel with isn't actually a number.
	ChanAmountNotNumber

	// ChannelTooLarge indicates that the amounts specified to fund the
	// channel with is greater than maxChannelSize.
	ChannelTooLarge

	// ChannelTooSmall indicates that the channel size required is below
	// minChannelSize.
	ChannelTooSmall

	// PushIncorrect indicates that the amount specified to push to the
	// other end of the channel is greater-than-or-equal-to the local
	// funding amount.
	PushIncorrect

	// ChannelOpenFail indicates some error occurred when attempting to
	// open a channel with the target peer.
	ChannelOpenFail

	// HaveChannel indicates that the faucet already has a channel open
	// with the target node.
	HaveChannel

	// HavePendingChannel indicates that the faucet already has a channel
	// pending with the target node.
	HavePendingChannel

	// ErrorGeneratingInvoice indicates that some error happened when generating
	// an invoice
	ErrorGeneratingInvoice

	// InvoiceAmountTooHigh indicates the user tried to generate an invoice
	// that was too expensive.
	InvoiceAmountTooHigh

	// ErrorDecodingPayReq indicates the user tried to input wrong invoice to create Payment Request
	ErrorDecodingPayReq

	// PaymentStreamError indicates that you get an error in the payment stream proccess
	PaymentStreamError

	// ErrorPaymentAmount indicates the user tried to pay an invoice with
	// a value greater then the limit
	ErrorPaymentAmount

	// TimeLimitError indicates the user tried to do an action without wait the timelimit
	TimeLimitError
)

// String returns a human readable string describing the chanCreationError.
// This string is used in the templates in order to display the error to the
// user.
func (c ChanCreationError) String() string {
	switch c {
	case NoError:
		return ""
	case InvalidAddress:
		return "Not a valid public key"
	case NotConnected:
		return "Faucet cannot connect to this node"
	case ChanAmountNotNumber:
		return "Amount must be a number"
	case ChannelTooLarge:
		return "Amount is too large"
	case ChannelTooSmall:
		return fmt.Sprintf("Minimum channel size is is %d DCR", minChannelSize)
	case PushIncorrect:
		return "Initial Balance is incorrect"
	case ChannelOpenFail:
		return "Faucet is not able to open a channel with this node"
	case HaveChannel:
		return "Faucet already has an active channel with this node"
	case HavePendingChannel:
		return "Faucet already has a pending channel with this node"
	case ErrorGeneratingInvoice:
		return "Error generating Invoice"
	case InvoiceAmountTooHigh:
		return "Invoice amount too high"
	case ErrorDecodingPayReq:
		return "Error decoding payment request"
	case PaymentStreamError:
		return "Error on payment request, try again"
	case ErrorPaymentAmount:
		return fmt.Sprintf("The amount of invoice exceeds the limit of %d Atoms", maxPaymentAtoms)
	case TimeLimitError:
		return "Action time limited. Please wait."

	default:
		return fmt.Sprintf("%v", uint8(c))
	}
}
