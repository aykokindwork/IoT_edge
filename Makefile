generate_rpc_go:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/classifier/classifier.proto

generate_rpc_python:
	python -m grpc_tools.protoc -I. --python_out=ml-service/ --grpc_python_out=ml-service ./proto/classifier/classifier.proto

generate: generate_rpc_go generate_rpc_python


