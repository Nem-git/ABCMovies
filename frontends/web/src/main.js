import { create, toJsonString } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { createGrpcWebTransport } from '@connectrpc/connect-web';

import {
  CoreService,
  GetJobRequestSchema,
  LoginRequestSchema,
  PasswordLoginSchema,
  PasswordSignUpSchema,
  SignUpRequestSchema,
  SubscribeRequestSchema,
} from './gen/abcmovies/api/v1/core_pb.js';
import { EventEnvelopeSchema } from './gen/abcmovies/core/v1/event_pb.js';

const log = (line) => {
  const el = document.getElementById('log');
  el.textContent += line + '\n';
  el.scrollTop = el.scrollHeight;
};

// The session token is held in memory only: a page reload requires login
// again, which keeps the M0 client free of storage-side token hygiene.
let token = null;

const authInterceptor = (next) => async (req) => {
  if (token) req.header.set('Authorization', `Bearer ${token}`);
  return next(req);
};

const transport = createGrpcWebTransport({
  baseUrl: window.location.origin,
  interceptors: [authInterceptor],
});
const client = createClient(CoreService, transport);

document.getElementById('signup').addEventListener('click', async () => {
  const username = document.getElementById('username').value;
  const password = document.getElementById('password').value;
  try {
    const res = await client.signUp(
      create(SignUpRequestSchema, {
        username,
        authMethod: {
          case: 'password',
          value: create(PasswordSignUpSchema, {
            password: new TextEncoder().encode(password),
          }),
        },
      }),
    );
    const recovery = document.getElementById('recovery');
    recovery.textContent = `Recovery key (shown once — store it safely): ${res.recoveryKey}`;
    recovery.classList.remove('hidden');
    log(`signed up: ${res.userId}`);
  } catch (err) {
    log(`sign up failed: ${err.message}`);
  }
});

document.getElementById('login').addEventListener('click', async () => {
  const username = document.getElementById('username').value;
  const password = document.getElementById('password').value;
  try {
    const res = await client.login(
      create(LoginRequestSchema, {
        username,
        authMethod: {
          case: 'password',
          value: create(PasswordLoginSchema, {
            password: new TextEncoder().encode(password),
          }),
        },
      }),
    );
    token = res.token;
    log('logged in; session token stored in memory');
  } catch (err) {
    log(`login failed: ${err.message}`);
  }
});

document.getElementById('getJob').addEventListener('click', async () => {
  try {
    const res = await client.getJob(
      create(GetJobRequestSchema, {
        jobId: document.getElementById('jobId').value,
      }),
    );
    if (!res.job) {
      log('job not found');
      return;
    }
    log(`job ${res.job.id}: status=${res.job.status} kind=${res.job.kind}`);
  } catch (err) {
    log(`get job failed: ${err.message}`);
  }
});

document.getElementById('subscribe').addEventListener('click', async () => {
  try {
    for await (const res of client.subscribe(
      create(SubscribeRequestSchema, {}),
    )) {
      const ev = res.event;
      if (ev.jobStatus) {
        log(`event: job ${ev.jobStatus.jobId} → ${ev.jobStatus.status}`);
      } else {
        log(`event: ${toJsonString(EventEnvelopeSchema, ev)}`);
      }
    }
  } catch (err) {
    log(`subscription closed: ${err.message}`);
  }
});
