images:
	docker build upstream -t chrlic/java-test-upstream:v1.0
	docker push chrlic/java-test-upstream:v1.0
	docker build downstream -t chrlic/java-test-downstream:v1.0
	docker push chrlic/java-test-downstream:v1.0

deploy:
	kubectl apply -f downstream/d-downstream.yaml
	kubectl apply -f upstream/d-upstream.yaml

undeploy:
	kubectl delete -f downstream/d-downstream.yaml
	kubectl delete -f upstream/d-upstream.yaml

redeploy: undeploy deploy