module Auditable
  extend ActiveSupport::Concern

  included do
    has_many :audit_logs, as: :auditable
    after_create :log_creation
    after_update :log_update
  end

  def audit_trail
    audit_logs.order(created_at: :desc)
  end

  def last_audited_at
    audit_logs.maximum(:created_at)
  end
end
