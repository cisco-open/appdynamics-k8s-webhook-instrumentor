images:
	docker build testserver -t chrlic/sample-nodejs-app:latest
	docker push chrlic/sample-nodejs-app:latest

deploy:
	kubectl apply -f testserver/d-testserver.yaml

undeploy:
	kubectl delete -f testserver/d-testserver.yaml

redeploy: undeploy deploy