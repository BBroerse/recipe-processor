package domain

import "errors"

// Domain sentinel errors for recipe processing.
var (
	ErrEmptyRecipeText = errors.New("recipe text cannot be empty")
	ErrRecipeNotFound  = errors.New("recipe not found")
	ErrLLMProcessing   = errors.New("llm processing failed")
	ErrTextTooLong     = errors.New("recipe text exceeds maximum length")
)
