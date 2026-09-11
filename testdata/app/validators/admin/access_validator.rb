module Admin
  class AccessValidator
    include ActiveModel::Validations

    attr_reader :role

    validates :role, presence: true
    validates :role, inclusion: { in: %w[admin superadmin] }

    def initialize(role)
      @role = role
    end
  end
end
