images:
	docker build MyFirstApp -t chrlic/sample-dotnet-app:latest
	docker push chrlic/sample-dotnet-app:latest

deploy:
	kubectl -ndemo apply -f MyFirstApp/d-myfirstapp.yaml

undeploy:
	kubectl -ndemo delete -f MyFirstApp/d-myfirstapp.yaml

redeploy: undeploy deploy