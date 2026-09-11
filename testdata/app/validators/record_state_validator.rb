class RecordStateValidator < ActiveModel::Validator
  ALLOWED_STATES = %w[draft published archived].freeze

  def validate(record)
    return if ALLOWED_STATES.include?(record.state)

    record.errors.add(:state, "must be one of #{ALLOWED_STATES.join(', ')}")
  end
end
