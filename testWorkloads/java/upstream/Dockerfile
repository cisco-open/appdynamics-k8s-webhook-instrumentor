# Pull base image.
FROM quay.io/centos/centos:stream8

# Install Java.
RUN yum update -y 
RUN yum install -y java-11-openjdk-devel

# Add binary source directory as a local working direstory
RUN mkdir /cmd
ADD upstream-0.0.1-SNAPSHOT.jar /cmd
ADD run_upstream.sh /cmd
WORKDIR /cmd
#make entrypoint shellscript executable
RUN chmod +x run_upstream.sh

# Define commonly used JAVA_HOME variable
ENV JAVA_HOME /usr/lib/jvm/jre-1.11.0-openjdk
ENV CLASSPATH $JAVA_HOME/lib

EXPOSE 8181

# Define entrypoint to start with.
ENTRYPOINT ["/cmd/run_upstream.sh"]

