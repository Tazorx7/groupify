import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiRequest } from '@/lib/api'
import type {
  CreateRegistrationRequest,
  RegistrationConfirmation,
  RegistrationStatus,
  TopicListResponse,
} from '@/lib/types'

export const studentKeys = {
  topics: ['public', 'topics'] as const,
  status: ['public', 'registration-status'] as const,
}

export function useTopics() {
  return useQuery({
    queryKey: studentKeys.topics,
    queryFn: () => apiRequest<TopicListResponse>('/topics'),
    // Availability changes as other students register, so this view should not
    // be served from a long-lived cache.
    staleTime: 10_000,
  })
}

export function useRegistrationStatus() {
  return useQuery({
    queryKey: studentKeys.status,
    queryFn: () => apiRequest<RegistrationStatus>('/registration-status'),
    staleTime: 10_000,
  })
}

interface SubmitArgs {
  values: CreateRegistrationRequest
  /**
   * Generated when the student enters the review step and reused for retries of
   * that exact payload, so a dropped connection cannot create two
   * registrations. It must change whenever the payload changes.
   */
  idempotencyKey: string
}

export function useRegister() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ values, idempotencyKey }: SubmitArgs) =>
      apiRequest<RegistrationConfirmation>('/registrations', {
        method: 'POST',
        body: values,
        headers: { 'Idempotency-Key': idempotencyKey },
      }),
    onSuccess: () => {
      // The student's own registration changed the counts.
      queryClient.invalidateQueries({ queryKey: studentKeys.topics })
    },
    onError: (error) => {
      // A conflict almost always means somebody else took the seat, so the
      // availability shown on screen is stale either way.
      if (error instanceof Error && 'status' in error) {
        queryClient.invalidateQueries({ queryKey: studentKeys.topics })
        queryClient.invalidateQueries({ queryKey: studentKeys.status })
      }
    },
  })
}
