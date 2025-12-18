INSERT INTO clients (
    client_id,
    client_secret_hash,
    rate_limit
)
VALUES (
    'bhim-psp',
    '$2a$12$rpY10LLoklRIPReccfX6feyfR8BDpSObE8UnzM/vVwChFB7iRB7xC',
    100
)
ON CONFLICT (client_id) DO UPDATE
SET client_secret_hash = EXCLUDED.client_secret_hash,
    rate_limit = EXCLUDED.rate_limit;


bhim-psp
bhim-uat-secret-key
$2a$12$rpY10LLoklRIPReccfX6feyfR8BDpSObE8UnzM/vVwChFB7iRB7xC


INSERT INTO tenants (
    id,
    external_tid,
    name
)
VALUES (
    '8C9ADDBB-0456-4F81-8FFF-E81146BED870',
    '1234567890',
    'Bhim PSP'
)
ON CONFLICT (id)
DO UPDATE SET
    external_tid = EXCLUDED.external_tid,
    name = EXCLUDED.name;




docker run -d --name session-redis -p 


docker run -d \
  --name bap-auth-service \
  --network completecertsandox_onix-network \
  -p 9090:9090 \
  -e JWT_PRIVATE_KEY='-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCngvQOyaCZxcUp
SNqWajD+MM+damMCvyTTmN5gyUAh0okxfzaUjyFdlkud5bMxPrkDVT6Y13HLhONG
3uRgqN1CP/i7/fj/G/NTEqgie2xGDCDDrFCwg5kj9Nfj2hcsK3yPd1AkKiwKjT92
3z+Q/c8A8bHDxPeJYhvKQEu9a1BKckbde2P7YC/AbcqIVo6eQwxzhnaf9iWY/Vll
Tp/gcU8Vnx5uUgSfF48bwt8J5LIgj6wyOwlWZi8XKuuvqU7W4+LzQKYk6XA2PWtP
AKgcObywNbioEGb4kJcRWrxbDbkRS0jfLgLn0RTsAUZpknxh9B0kZHWRKGy8wXYV
qj840mjjAgMBAAECggEABQhnoBZEqgoWAW310IySJqVHU0ujQOiuIYjrOYO/9BpY
5lg7ULDzb0Jbehl/sGxZ0ddQ7/XOX8z3RmCjeKH944ZoIbWDKLJpjhfnmTxkTzVj
ExVXQA5q6G6w/HRbgEKUhugG/TEFLVc2iYDjs03bCHC6kNPvuzyJVSKjyNdWEVcb
j0hVdoB7MHugpXjG4cf2z3cTjtiWy8/7w4YHkEOQo6XttucRXY75+V/PjrmDjReb
4DJVsWLqgp8L2kPuA3GHPqzQiKcpzFyF6tOpQjl8Xo0sxPN0soSiY24/5CiD52vY
7AmdK/yHJCzDlmI9lBXaGIdEXcAAeDG5+1fFBXqo4QKBgQDWdmeZ+t85IfB1FHji
LAEvfBgvumtoYGoDAL+pslnV30WutVR+VlyMYvWHthPbQRe729kFp/Ypgo7CZVIV
IaKFg00SzMgExWfyPWTX5B9uXAkIEeLZpYQGElEwfi0+o4cROJXfr1S8im3zz09f
mt9z056AkV0sf6my7TH3bVi8gwKBgQDH9JmXGS2uK/PDeG5r+PSX/S5ZAiz2JPUP
5mcqm2VJ5mSKVQiruKl4PN4znos2MYVN5ZgwZFsu5lIVG5/cfEOrtWFqBPoP6yn+
tNMK6ixhanN0ernZ+awYGJJkkilm+umfFLC7lmTYqY4Idg0NUuNu2vl+6lqgW4fC
qLWt2PK0IQKBgBqxYBG1POViiQg5hRY5fehIHMaMAGRcY7V9+V0Ius+423Z0UVDs
NNawVnkOu4f1oRubsHZYwnXGLziY3c+NgSn2/rfRTy/w1hA7ffq1BQh6YhFkEIUg
ab9Ltlk/yyfZuKz3CwhtTTGuVSMccXen0hobg8Xi0eMA/MEtbqOqM3o7AoGAG949
Yc/CjBnYGZA5Y5cJD/3bbdBdz9iKxzKHgmqyDUCtFpKPaM+N3xIsrekU4fK474hm
U6hJBRpYqlR1TVeMXuwirZIQABP4gGVXXJgSo2kgukU4jea8U4dpL9cnKhEiameJ
0js9xuyqvQcm/opk5FhkmYm0I9Fd9IVq/NXVzcECgYANSm1L3+TPH3z5wW8K+/AY
U2KSWxtoO9PbrPmGVzypdU+4zCLE1cSOCvwaNDqw4yJxMafB/v6cG2a1XWvIouJu
jiutt/wycepISV2nr8use4d+OV1pqNeHKaTre6DVRp50sn53zYnkmxR6GBNSOk0p
Jwx3T1GX/fmtsaXN+KV34A==
-----END PRIVATE KEY-----' \
  -e JWT_PUBLIC_KEY='-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAp4L0DsmgmcXFKUjalmow
/jDPnWpjAr8k05jeYMlAIdKJMX82lI8hXZZLneWzMT65A1U+mNdxy4TjRt7kYKjd
Qj/4u/34/xvzUxKoIntsRgwgw6xQsIOZI/TX49oXLCt8j3dQJCosCo0/dt8/kP3P
APGxw8T3iWIbykBLvWtQSnJG3Xtj+2AvwG3KiFaOnkMMc4Z2n/YlmP1ZZU6f4HFP
FZ8eblIEnxePG8LfCeSyII+sMjsJVmYvFyrrr6lO1uPi80CmJOlwNj1rTwCoHDm8
sDW4qBBm+JCXEVq8Ww25EUtI3y4C59EU7AFGaZJ8YfQdJGR1kShsvMF2Fao/ONJo
4wIDAQAB
-----END PUBLIC KEY-----' \
  -e POSTGRES_USER=auth_user \
  -e POSTGRES_PASSWORD=auth_password \
  -e POSTGRES_DB=sessiondb \
  -e SERVER_PORT=9090 \
  imarinzone/bap-auth-service:latest



  CREATE USER user WITH PASSWORD 'password';


  docker run -d \
  --name postgres \
  --network completecertsandox_onix-network \
  -p 5435:5435 \
  -e POSTGRES_USER=auth_user \
  -e POSTGRES_PASSWORD=auth_password \
  -e POSTGRES_DB=sessiondb \
  postgres:15-alpine


  docker run -d \
  --name redis \
  --network completecertsandox_onix-network \
  -p 6382:6382 \
  redis:7-alpine \
  redis-server --port 6382