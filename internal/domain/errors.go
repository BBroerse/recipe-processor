package domain

import "errors"

var (
	ErrEmptyRecipeText = errors.New("recipe text cannot be empty")
	ErrRecipeNotFound  = errors.New("recipe not found")
	ErrLLMProcessing   = errors.New("llm processing failed")
	ErrTextTooLong     = errors.New("recipe text exceeds maximum length")
)
