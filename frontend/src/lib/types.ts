// Shared API types. These mirror docs/api.md exactly; if the backend contract
// changes, this file is the one place the client has to follow.

export interface Topic {
  id: number
  name: string
  description: string
  capacity: number
  registered_count: number
  remaining_spaces: number
  status: 'available' | 'full'
}

export interface TopicListResponse {
  topics: Topic[]
}

export interface RegistrationStatus {
  registration_open: boolean
}

export interface CreateRegistrationRequest {
  full_name: string
  index_number: string
  topic_id: number
}

export interface RegistrationConfirmation {
  registration_id: number
  full_name: string
  index_number: string
  topic: { id: number; name: string }
  registered_at: string
}

export interface Admin {
  id: number
  email: string
}

export interface AdminSession {
  admin: Admin
  csrf_token: string
  expires_at: string
}

export interface AdminTopic {
  id: number
  name: string
  description: string
  capacity: number
  current_members: number
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface AdminTopicListResponse {
  topics: AdminTopic[]
}

export interface Overview {
  total_registrations: number
  total_topics: number
  active_topics: number
  full_topics: number
  total_available_spaces: number
  registration_open: boolean
}

export interface AdminRegistration {
  id: number
  full_name: string
  index_number: string
  topic_id: number
  topic_name: string
  created_at: string
}

export interface AdminRegistrationListResponse {
  items: AdminRegistration[]
  total: number
  limit: number
  offset: number
}

export type RegistrationSort = 'registered_at' | 'full_name' | 'index_number' | 'topic'
export type SortDirection = 'asc' | 'desc'

export interface RegistrationFilters {
  search: string
  topicId: number | null
  sort: RegistrationSort
  dir: SortDirection
  page: number
  pageSize: number
}

export interface Settings {
  registration_open: boolean
  updated_at: string
  updated_by_admin_id: number | null
}

export interface AuditEntry {
  id: number
  actor_admin_id: number | null
  actor_email: string | null
  actor_label: string | null
  action: string
  entity_type: string
  entity_id: number | null
  before_data: Record<string, unknown> | null
  after_data: Record<string, unknown> | null
  created_at: string
}

export interface AuditListResponse {
  items: AuditEntry[]
  total: number
  limit: number
  offset: number
}
