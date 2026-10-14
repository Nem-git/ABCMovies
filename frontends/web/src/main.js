import { createClient } from '@connectrpc/connect';
import { createGrpcWebTransport } from '@connectrpc/connect-web';

import { CoreService } from './gen/abcmovies/api/v1/core_pb.js';
import { createApp } from './app.js';

let currentToken = null;

const authInterceptor = (next) => async (req) => {
  if (currentToken) req.header.set('Authorization', `Bearer ${currentToken}`);
  return next(req);
};

const transport = createGrpcWebTransport({
  baseUrl: window.location.origin,
  interceptors: [authInterceptor],
});

const client = createClient(CoreService, transport);

createApp({
  client,
  doc: document,
  onTokenChange: (token) => {
    currentToken = token;
  },
});
