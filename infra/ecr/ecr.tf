resource "aws_ecr_repository" "image_storage" {
  name                 = "CYDERES"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}
