class SyncUserJob < ApplicationJob
  queue_as :default
  retry_on StandardError, wait: 5.seconds, attempts: 3
  discard_on ActiveJob::DeserializationError
  include Retryable

  def perform(user)
  end

  private def cleanup
  end
end