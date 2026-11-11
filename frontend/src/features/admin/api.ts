import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiRequest, setCsrfToken, toQuery } from '@/lib/api'
import type {
  AdminRegistrationListResponse,
  AdminSession,
  AdminTopic,
  AdminTopicListResponse,
  AuditListResponse,
  Overview,
  RegistrationFilters,
  Settings,
} from '@/lib/types'

export const adminKeys = {
  session: ['admin', 'session'] as const,
  overview: ['admin', 'overview'] as const,
  topics: ['admin', 'topics'] as const,
  settings: ['admin', 'settings'] as const,
  registrations: (filters: RegistrationFilters) => ['admin', 'registrations', filters] as const,
  audit: (action: string, page: number) => ['admin', 'audit', action, page] as const,
}

// ---------------------------------------------------------------------------
// Session
// ---------------------------------------------------------------------------

/**
 * Recovers the session (and the CSRF token) after a page load.
 *
 * `retry: false` matters: a 401 here is a normal state (signed out), not a
 * transient failure worth retrying.
 */
export function useAdminSession() {
  return useQuery({
    queryKey: adminKeys.session,
    queryFn: async () => {
      const session = await apiRequest<AdminSession>('/admin/session')
      setCsrfToken(session.csrf_token)
      return session
    },
    retry: false,
    staleTime: Infinity,
  })
}

export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (credentials: { email: string; password: string }) =>
      apiRequest<AdminSession>('/admin/login', { method: 'POST', body: credentials }),
    onSuccess: (session) => {
      setCsrfToken(session.csrf_token)
      queryClient.setQueryData(adminKeys.session, session)
      void queryClient.invalidateQueries()
    },
  })
}

export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => apiRequest<void>('/admin/logout', { method: 'POST' }),
    onSettled: () => {
      setCsrfToken(null)
      queryClient.clear()
    },
  })
}

// ---------------------------------------------------------------------------
// Overview / topics
// ---------------------------------------------------------------------------

export function useOverview() {
  return useQuery({
    queryKey: adminKeys.overview,
    queryFn: () => apiRequest<Overview>('/admin/overview'),
  })
}

export function useAdminTopics() {
  return useQuery({
    queryKey: adminKeys.topics,
    queryFn: () => apiRequest<AdminTopicListResponse>('/admin/topics'),
  })
}

interface TopicInput {
  name: string
  description: string
  capacity: number
}

function invalidateTopicData(queryClient: ReturnType<typeof useQueryClient>) {
  void queryClient.invalidateQueries({ queryKey: adminKeys.topics })
  void queryClient.invalidateQueries({ queryKey: adminKeys.overview })
  void queryClient.invalidateQueries({ queryKey: ['public'] })
}

export function useCreateTopic() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: TopicInput) =>
      apiRequest<AdminTopic>('/admin/topics', { method: 'POST', body: input }),
    onSuccess: () => invalidateTopicData(queryClient),
  })
}

export function useUpdateTopic() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, patch }: { id: number; patch: Partial<TopicInput> }) =>
      apiRequest<AdminTopic>(`/admin/topics/${id}`, { method: 'PATCH', body: patch }),
    onSuccess: () => invalidateTopicData(queryClient),
  })
}

export function useSetTopicActive() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, active }: { id: number; active: boolean }) =>
      apiRequest<AdminTopic>(`/admin/topics/${id}/${active ? 'activate' : 'deactivate'}`, {
        method: 'POST',
      }),
    onSuccess: () => invalidateTopicData(queryClient),
  })
}

// ---------------------------------------------------------------------------
// Registrations
// ---------------------------------------------------------------------------

export function useRegistrations(filters: RegistrationFilters) {
  return useQuery({
    queryKey: adminKeys.registrations(filters),
    queryFn: () =>
      apiRequest<AdminRegistrationListResponse>(
        `/admin/registrations${toQuery({
          search: filters.search,
          topic_id: filters.topicId,
          sort: filters.sort,
          dir: filters.dir,
          limit: filters.pageSize,
          offset: (filters.page - 1) * filters.pageSize,
        })}`,
      ),
    placeholderData: keepPreviousData,
  })
}

export function useDeleteRegistration() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => apiRequest<void>(`/admin/registrations/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin', 'registrations'] })
      void queryClient.invalidateQueries({ queryKey: adminKeys.overview })
      void queryClient.invalidateQueries({ queryKey: adminKeys.topics })
      void queryClient.invalidateQueries({ queryKey: ['admin', 'audit'] })
    },
  })
}

// ---------------------------------------------------------------------------
// Settings / audit
// ---------------------------------------------------------------------------

export function useAdminSettings() {
  return useQuery({
    queryKey: adminKeys.settings,
    queryFn: () => apiRequest<Settings>('/admin/settings'),
  })
}

export function useUpdateSettings() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (registrationOpen: boolean) =>
      apiRequest<Settings>('/admin/settings', {
        method: 'PATCH',
        body: { registration_open: registrationOpen },
      }),
    onSuccess: (settings) => {
      queryClient.setQueryData(adminKeys.settings, settings)
      void queryClient.invalidateQueries({ queryKey: adminKeys.overview })
      void queryClient.invalidateQueries({ queryKey: ['public'] })
      void queryClient.invalidateQueries({ queryKey: ['admin', 'audit'] })
    },
  })
}

export function useAuditLogs(action: string, page: number, pageSize = 50) {
  return useQuery({
    queryKey: adminKeys.audit(action, page),
    queryFn: () =>
      apiRequest<AuditListResponse>(
        `/admin/audit-logs${toQuery({
          action,
          limit: pageSize,
          offset: (page - 1) * pageSize,
        })}`,
      ),
    placeholderData: keepPreviousData,
  })
}
