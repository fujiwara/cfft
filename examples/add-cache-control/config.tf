provider "aws" {
  region = "us-east-1"
}

terraform {
  required_providers {
    aws = {
      source = "hashicorp/aws"
    }
  }
}

resource "aws_cloudfront_function" "my-function" {
  name    = data.external.my-function.result["name"]
  runtime = data.external.my-function.result["runtime"]
  code    = data.external.my-function.result["code"]
  comment = data.external.my-function.result["comment"]
  publish = true
}

data "external" "my-function" {
  program = ["cfft", "tf", "--external"]
}

/*
import {
  to = aws_cloudfront_function.my-function
  id = "my-function"
}
*/
