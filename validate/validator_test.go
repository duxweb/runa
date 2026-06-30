package validate

import (
	"errors"
	"testing"
)

type sampleInput struct {
	Name  string
	Age   int
	Email string
}

type privateInput struct {
	Name   string
	secret string
}

func TestValidatorRules(t *testing.T) {
	input := &sampleInput{Name: "Runa", Age: 18, Email: "runa@example.com"}
	validator := New(input, nil)
	validator.Field("Name").Required("请输入名称").MinLen(2, "名称太短")
	validator.Field("Age").Min(18, "年龄太小").Max(99, "年龄太大")
	validator.Field("Email").Email("邮箱格式错误")

	if err := validator.Run(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestValidatorReturnsFieldError(t *testing.T) {
	input := &sampleInput{}
	validator := New(input, nil)
	validator.Field("Name").Required("请输入名称")

	err := validator.Run()
	if err == nil {
		t.Fatal("expected validation error")
	}
	validationErr := AsError(err)
	if validationErr == nil || len(validationErr.Errors) != 1 {
		t.Fatalf("validation error = %#v", validationErr)
	}
	if validationErr.Errors[0].Field != "Name" || validationErr.Errors[0].Message != "请输入名称" {
		t.Fatalf("field error = %#v", validationErr.Errors[0])
	}
}

func TestValidatorCallbacks(t *testing.T) {
	input := &sampleInput{Name: "runa"}
	validator := New(input, "ctx")
	validator.Field("Name").Call(func(ctx string, value string) error {
		if ctx != "ctx" || value != "runa" {
			return errors.New("invalid callback args")
		}
		return nil
	})
	validator.Check(func(ctx string) error {
		if ctx != "ctx" {
			return errors.New("invalid check args")
		}
		return nil
	})

	if err := validator.Run(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestValidatorCallbackIgnoresNonErrorReturn(t *testing.T) {
	input := &sampleInput{Name: "runa"}
	validator := New(input, nil)
	validator.Field("Name").Call(func(value string) bool {
		return value == "runa"
	})

	if err := validator.Run(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestValidatorMinMaxRejectNonNumericString(t *testing.T) {
	input := &sampleInput{Name: "abc"}
	validator := New(input, nil)
	validator.Field("Name").Min(1, "必须是数字")

	if err := validator.Run(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidatorLengthUsesRunes(t *testing.T) {
	input := &sampleInput{Name: "中"}
	validator := New(input, nil)
	validator.Field("Name").MaxLen(1, "最多一个字符")

	if err := validator.Run(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestValidatorUnexportedFieldDoesNotPanic(t *testing.T) {
	input := &privateInput{Name: "runa", secret: "hidden"}
	validator := New(input, nil)
	validator.Field("secret").Required("secret required")

	if err := validator.Run(); err == nil {
		t.Fatal("expected validation error")
	}
}
