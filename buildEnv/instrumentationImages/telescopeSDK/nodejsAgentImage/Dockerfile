FROM node:lts-slim

RUN mkdir /opt/telescope
COPY opt/telescope /opt/telescope
WORKDIR /opt/telescope

RUN npm install \
  @cisco-telescope/cisco-sdk-node \
  process
  
RUN chmod a+rwx /opt/telescope

CMD ["cat", "Just delivering the Node.js agent for Cisco Telescope"]