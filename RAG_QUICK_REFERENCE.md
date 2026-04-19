# RAG Chatbot - API Quick Reference

## 🎯 All Endpoints at a Glance

| Method | Endpoint | Purpose | Auth |
|--------|----------|---------|------|
| **POST** | `/api/chat/rag` | Chat with AI | None |
| **POST** | `/api/chat/rag/clear` | Clear conversation | None |
| **POST** | `/api/chat/rag/delete` | Delete conversation | None |
| **POST** | `/api/admin/rag/ingest` | Re-ingest data | Admin |
| **GET** | `/health` | Server status | None |

---

## 1️⃣ Chat Endpoint

```
POST /api/chat/rag
```

**Request:**
```json
{
  "question": "What events are in Brooklyn?",
  "user_id": "user_123"
}
```

**Success Response (200):**
```json
{
  "answer": "We have several events in Brooklyn...",
  "sources": ["events", "blogs"],
  "conversation_id": "user_123",
  "success": true
}
```

**Fields:**
- `answer` (string) - AI-generated answer
- `sources` (array) - Where the answer came from
- `conversation_id` (string) - Unique conversation identifier
- `success` (boolean) - Whether request succeeded
- `error` (string, optional) - Error message if failed

---

## 2️⃣ Clear Chat

```
POST /api/chat/rag/clear
```

**Request:**
```json
{
  "user_id": "user_123"
}
```

**Response (200):**
```json
{
  "message": "Conversation cleared",
  "status": "success"
}
```

**When to use:** Start fresh conversation, user wants to reset chat history

---

## 3️⃣ Delete Chat

```
POST /api/chat/rag/delete
```

**Request:**
```json
{
  "user_id": "user_123"
}
```

**Response (200):**
```json
{
  "message": "Conversation deleted",
  "status": "success"
}
```

**When to use:** User logs out or deletes account

---

## 4️⃣ Ingest Data (Admin)

```
POST /api/admin/rag/ingest
```

**Headers:**
```
Authorization: Bearer <admin-token>
```

**Response (200):**
```json
{
  "success": true,
  "stats": {
    "total_documents": 2000,
    "chunks": 4500,
    "vectors_stored": 4500,
    "errors": 0,
    "duration": "2m31s"
  }
}
```

**When to use:** After database updates, scheduled daily/weekly

---

## 5️⃣ Health Check

```
GET /health
```

**Response (200):**
```
ok
```

---

## 📱 Frontend Integration Example

### React/JavaScript

```javascript
// Chat with RAG
async function chatWithRAG(question, userId) {
  const response = await fetch('/api/chat/rag', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      question,
      user_id: userId
    })
  });
  
  const data = await response.json();
  
  if (data.success) {
    console.log('Answer:', data.answer);
    console.log('Sources:', data.sources);
  } else {
    console.error('Error:', data.error);
  }
  
  return data;
}

// Clear conversation
async function clearChat(userId) {
  await fetch('/api/chat/rag/clear', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ user_id: userId })
  });
}
```

### Swift/iOS

```swift
// Chat request
let request = URLRequest(url: URL(string: "http://localhost:5000/api/chat/rag")!)
var request = request
request.httpMethod = "POST"
request.setValue("application/json", forHTTPHeaderField: "Content-Type")

let body = ["question": "What events?", "user_id": "user_123"]
request.httpBody = try JSONSerialization.data(withJSONObject: body)

URLSession.shared.dataTask(with: request) { data, response, error in
    let json = try JSONSerialization.jsonObject(with: data!) as? [String: Any]
    print(json?["answer"] ?? "No answer")
}.resume()
```

### Flutter/Dart

```dart
// Chat request
final response = await http.post(
  Uri.parse('http://localhost:5000/api/chat/rag'),
  headers: {'Content-Type': 'application/json'},
  body: jsonEncode({
    'question': 'What events are available?',
    'user_id': 'user_123',
  }),
);

if (response.statusCode == 200) {
  final data = jsonDecode(response.body);
  print(data['answer']);
  print(data['sources']);
}
```

---

## 🔄 Flow Diagram

```
Frontend User Input
        ↓
POST /api/chat/rag
        ↓
Go Server (RAG Engine)
├─ Search Pinecone for similar content
├─ Call OpenAI GPT-4 to generate answer
└─ Store in conversation memory
        ↓
Return Response { answer, sources, success }
        ↓
Display to User
```

---

## ⏱️ Response Times

| Operation | Time |
|-----------|------|
| Embedding (OpenAI) | ~200ms |
| Vector Search (Pinecone) | ~100ms |
| LLM Generation (GPT-4) | ~2-3s |
| **Total** | **~2.3-3.3s** |

---

## 🛡️ Error Codes

| Code | Meaning | Example |
|------|---------|---------|
| **200** | Success | Answer generated |
| **400** | Bad Request | Missing question field |
| **500** | Server Error | OpenAI API down |

---

## 💰 Cost Estimate

**Per 1000 API calls (1000 questions):**

| Service | Cost | Notes |
|---------|------|-------|
| OpenAI Embeddings | $0.02 | text-embedding-3-small |
| OpenAI LLM | $5-15 | gpt-4-turbo (~1500 tokens/query) |
| Pinecone | FREE | Up to 100K vectors free tier |
| **Total** | **$5-15** | Can reduce with gpt-3.5-turbo |

**Optimization options:**
- Use `gpt-3.5-turbo` instead of `gpt-4-turbo` (80% cheaper)
- Cache common questions
- Reduce `TOP_K_RETRIEVAL` from 4 to 2
- Batch ingest operations

---

## 🧪 Testing with cURL

### Quick Test
```bash
# Test chat
curl -X POST http://localhost:5000/api/chat/rag \
  -H "Content-Type: application/json" \
  -d '{
    "question": "What events are available?",
    "user_id": "user_123"
  }' | jq

# Clear conversation
curl -X POST http://localhost:5000/api/chat/rag/clear \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user_123"}' | jq

# Health check
curl http://localhost:5000/health
```

---

## 📚 Knowledge Base Contents

Data ingested from:
1. **events** table
   - Title, category, location, description
   - ~1000 documents

2. **users** table (anonymized)
   - Bio, location, member since
   - ~500 profiles

3. **blogs** table
   - Title, category, content
   - ~500 articles

**Total:** ~2000 documents → ~4500 chunks → ~4500 vectors

---

## 🚀 Production Checklist

- [ ] Set `OPENAI_API_KEY` in environment
- [ ] Set `PINECONE_API_KEY` in environment
- [ ] Create Pinecone index
- [ ] Run data ingestion
- [ ] Add authentication to `/api/admin/rag/ingest`
- [ ] Add rate limiting to `/api/chat/rag`
- [ ] Enable CORS for frontend domains
- [ ] Set up error logging & monitoring
- [ ] Test all endpoints
- [ ] Document custom queries for team

---

**Ready to use! Start chatting!** 🎉
